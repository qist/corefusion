package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"corefusion/internal/constants"
)

// EnsureCert loads an existing mTLS certificate or bootstraps a new one from the Panel.
func (a *AgentApp) EnsureCert() error {
	if a.hasValidCert() {
		log.Println("[Agent] using existing mTLS certificate")
		return nil
	}

	if a.cfg.Bootstrap.Token == "" {
		return fmt.Errorf("no bootstrap token and no valid cert — configure bootstrap.token in config.yaml")
	}

	log.Println("[Agent] bootstrapping with panel...")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "agent-" + strconv.FormatInt(a.cfg.Panel.ServerID, 10)
	}

	scheme := "http"
	if a.cfg.Panel.TLS {
		scheme = "https"
	}
	bootstrapURL := fmt.Sprintf("%s://%s:%d/api/v1/agent/bootstrap", scheme, a.cfg.Panel.Host, a.cfg.Panel.Port)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"server_id":       a.cfg.Panel.ServerID,
		"bootstrap_token": a.cfg.Bootstrap.Token,
		"hostname":        hostname,
		"agent_version":   constants.Version,
	})

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Bootstrap phase: no CA cert yet
		},
	}
	resp, err := client.Post(bootstrapURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("bootstrap request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("bootstrap failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Code string `json:"code"`
		Data struct {
			CertPEM   string `json:"cert_pem"`
			KeyPEM    string `json:"key_pem"`
			CACertPEM string `json:"ca_cert_pem"`
			ExpiresAt int64  `json:"cert_expires_at"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse bootstrap response: %w", err)
	}
	if apiResp.Code != "" && apiResp.Code != "OK" {
		return fmt.Errorf("bootstrap error: %s - %s", apiResp.Code, apiResp.Message)
	}

	if err := os.WriteFile(a.certPath, []byte(apiResp.Data.CertPEM), 0640); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(a.keyPath, []byte(apiResp.Data.KeyPEM), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	if err := os.WriteFile(a.caPath, []byte(apiResp.Data.CACertPEM), 0640); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}

	log.Printf("[Agent] bootstrap successful, cert expires at %v", time.Unix(apiResp.Data.ExpiresAt, 0))
	return nil
}

// hasValidCert checks whether the on-disk mTLS certificate is present and not expired.
func (a *AgentApp) hasValidCert() bool {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		return false
	}
	if _, err := os.ReadFile(a.caPath); err != nil {
		return false
	}
	for _, c := range cert.Certificate {
		parsed, err := x509.ParseCertificate(c)
		if err != nil {
			continue
		}
		if time.Now().After(parsed.NotAfter) {
			return false
		}
	}
	return true
}
