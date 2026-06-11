package warp

import (
	"bytes"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// warpAPIResponse is the Cloudflare WARP API registration response.
type warpAPIResponse struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Account struct {
		ID          string `json:"id"`
		AccountType string `json:"account_type"`
		License     string `json:"license"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				Host string `json:"host"`
			} `json:"endpoint"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

// WarpStatus contains WARP registration status info.
type WarpStatus struct {
	Status      string `json:"status"`
	Registered  bool   `json:"registered"`
	DeviceID    string `json:"device_id,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	AccountType string `json:"account_type,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	Reserved    string `json:"reserved,omitempty"`
	AddressIPv4 string `json:"address_ipv4,omitempty"`
	AddressIPv6 string `json:"address_ipv6,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	LicenseKey  string `json:"license_key,omitempty"`
}

// Service handles WARP business logic.
type Service struct {
	repo    *repository.WarpRepository
	dataDir string
}

// NewService creates a new WARP service.
func NewService(repo *repository.WarpRepository, dataDir string) *Service {
	return &Service{repo: repo, dataDir: dataDir}
}

// GetStatus returns the current WARP registration status.
func (s *Service) GetStatus() (*WarpStatus, error) {
	cfg, err := s.repo.GetActive(nil)
	if err != nil {
		return &WarpStatus{Status: "not_registered", Registered: false}, nil
	}
	return &WarpStatus{
		Status:      cfg.Status,
		Registered:  cfg.Status != "not_registered",
		DeviceID:    cfg.DeviceID,
		AccountID:   cfg.AccountID,
		AccountType: cfg.AccountType,
		PrivateKey:  cfg.PrivateKey,
		PublicKey:   cfg.PublicKey,
		Reserved:    cfg.Reserved,
		AddressIPv4: cfg.AddressIPv4,
		AddressIPv6: cfg.AddressIPv6,
		Endpoint:    cfg.Endpoint,
		LicenseKey:  cfg.LicenseKey,
	}, nil
}

// Register registers with Cloudflare WARP API (pure Go, no external binary).
func (s *Service) Register() (*WarpStatus, error) {
	// Generate Curve25519 key pair
	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	// Generate random install ID and FCM token
	installID := randomString(22)
	fcmToken := randomString(134)

	payload := fmt.Sprintf(`{
		"key": "%s",
		"install_id": "%s",
		"fcm_token": "%s:APA91b%s",
		"tos": "%s",
		"model": "Android",
		"serial_number": "%s",
		"locale": "zh_CN"
	}`, publicKey, installID, installID, fcmToken,
		time.Now().UTC().Format("2006-01-02T15:04:05.999Z"), installID)

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS12,
			},
		},
	}

	req, err := http.NewRequest("POST", "https://api.cloudflareclient.com/v0a2158/reg", bytes.NewBufferString(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("CF-Client-Version", "a-6.10-2158")
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registration failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var apiResp warpAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Decode client_id to reserved values
	reserved, err := decodeClientID(apiResp.Config.ClientID)
	if err != nil {
		return nil, fmt.Errorf("decode client_id: %w", err)
	}

	// Extract peer info
	var peerPublicKey, endpoint string
	var addressV4, addressV6 string
	if len(apiResp.Config.Peers) > 0 {
		peerPublicKey = apiResp.Config.Peers[0].PublicKey
		endpoint = apiResp.Config.Peers[0].Endpoint.Host
	}
	addressV4 = apiResp.Config.Interface.Addresses.V4
	addressV6 = apiResp.Config.Interface.Addresses.V6

	cfg := &entity.WarpConfig{
		DeviceID:       apiResp.ID,
		Token:          apiResp.Token,
		PrivateKey:     privateKey,
		PublicKey:      peerPublicKey,
		Reserved:       reserved,
		AddressIPv4:    addressV4,
		AddressIPv6:    addressV6,
		Endpoint:       endpoint,
		ClientID:       apiResp.Config.ClientID,
		AccountID:      apiResp.Account.ID,
		AccountType:    apiResp.Account.AccountType,
		LicenseKey:     apiResp.Account.License,
		RegistrationAt: time.Now(),
		Status:         "registered",
	}

	if err := s.repo.Create(nil, cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &WarpStatus{
		Status:      cfg.Status,
		Registered:  true,
		DeviceID:    cfg.DeviceID,
		AccountID:   cfg.AccountID,
		AccountType: cfg.AccountType,
		PrivateKey:  cfg.PrivateKey,
		PublicKey:   cfg.PublicKey,
		Reserved:    cfg.Reserved,
		AddressIPv4: cfg.AddressIPv4,
		AddressIPv6: cfg.AddressIPv6,
		Endpoint:    cfg.Endpoint,
		LicenseKey:  cfg.LicenseKey,
	}, nil
}

// GenerateConfig generates a WireGuard config from the stored credentials.
func (s *Service) GenerateConfig() (string, error) {
	cfg, err := s.repo.GetActive(nil)
	if err != nil {
		return "", fmt.Errorf("no WARP registration found")
	}

	wgConfig := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32, %s/128
DNS = 1.1.1.1, 2606:4700:4700::1111

[Peer]
PublicKey = %s
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = %s
`, cfg.PrivateKey, cfg.AddressIPv4, cfg.AddressIPv6, cfg.PublicKey, cfg.Endpoint)

	runtimeDir := filepath.Join(s.dataDir, "runtime")
	os.MkdirAll(runtimeDir, 0750)
	configPath := filepath.Join(runtimeDir, "warp.conf")
	if err := os.WriteFile(configPath, []byte(wgConfig), 0600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	cfg.ConfigJSON = wgConfig
	cfg.Status = "active"
	if err := s.repo.Update(nil, cfg); err != nil {
		return "", fmt.Errorf("update config record: %w", err)
	}

	return wgConfig, nil
}

// GetConfig returns the current WARP WireGuard config.
func (s *Service) GetConfig() (string, error) {
	cfg, err := s.repo.GetActive(nil)
	if err != nil {
		return "", nil
	}
	return cfg.ConfigJSON, nil
}

// Reset clears the WARP configuration.
func (s *Service) Reset() error {
	cfg, err := s.repo.GetActive(nil)
	if err != nil {
		return nil
	}
	configPath := filepath.Join(s.dataDir, "runtime", "warp.conf")
	os.Remove(configPath)
	return s.repo.Delete(nil, cfg.ID)
}

// generateKeyPair generates a Curve25519 key pair for WireGuard.
func generateKeyPair() (privateKeyBase64, publicKeyBase64 string, err error) {
	b := make([]byte, curve25519.ScalarSize)
	if _, err := crand.Read(b); err != nil {
		return "", "", err
	}
	// Clamp per Curve25519 / WireGuard spec
	b[0] &= 248
	b[31] &= 127
	b[31] |= 64

	pub, err := curve25519.X25519(b, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(b), base64.StdEncoding.EncodeToString(pub), nil
}

// decodeClientID decodes the base64 client_id to reserved values.
func decodeClientID(clientID string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(clientID)
	if err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(decoded)

	var reserved []string
	for i := 0; i < len(hexStr); i += 2 {
		hexByte := hexStr[i : i+2]
		decValue, _ := strconv.ParseInt(hexByte, 16, 64)
		reserved = append(reserved, fmt.Sprintf("%d", decValue))
	}
	return "[" + strings.Join(reserved, ",") + "]", nil
}

// randomString generates a random alphanumeric string of length n.
func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
