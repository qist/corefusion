package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"corefusion/internal/auth/jwt"
	"corefusion/internal/store/repository"
)

// InboundManager implements InboundUserManager using the inbound config repository
// and calls xray control API for live user management.
type InboundManager struct {
	inboundRepo *repository.InboundConfigRepository
	apiBaseURL  string
	jwtSecret   string
}

// NewInboundManager creates a new InboundManager.
func NewInboundManager(inboundRepo *repository.InboundConfigRepository, apiPort, jwtSecret string) *InboundManager {
	return &InboundManager{
		inboundRepo: inboundRepo,
		apiBaseURL:  "http://127.0.0.1:" + apiPort,
		jwtSecret:   jwtSecret,
	}
}

// AddUserToInbound adds a user entry to the inbound config JSON and calls xray API.
func (m *InboundManager) AddUserToInbound(ctx context.Context, inboundTag, email, uuid, password, flow, method string) error {
	// Find inbound by tag
	inbound, err := m.inboundRepo.GetByTag(ctx, 1, inboundTag)
	if err != nil {
		return fmt.Errorf("inbound not found: %s", inboundTag)
	}

	// Add to config JSON
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no settings in config")
	}

	protocol := detectProtocol(inboundTag)
	user := map[string]interface{}{
		"email":   email,
		"_paused": false,
	}

	switch protocol {
	case "vless":
		user["id"] = uuid
		user["flow"] = flow
		user["level"] = 0
	case "vmess":
		user["id"] = uuid
		user["alterId"] = 0
		user["level"] = 0
	case "trojan":
		user["password"] = password
		user["level"] = 0
	case "hysteria":
		user["auth"] = password
		user["level"] = 0
	case "shadowsocks":
		user["password"] = password
		user["method"] = method
		user["level"] = 0
	default:
		user["id"] = uuid
		user["password"] = password
		user["level"] = 0
	}

	for _, key := range []string{"clients", "users"} {
		if arr, ok := settings[key].([]interface{}); ok {
			settings[key] = append(arr, user)
			break
		}
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	if err := m.inboundRepo.Update(ctx, inbound); err != nil {
		return err
	}

	// Call xray control API to add user to running instance
	go m.callXrayAddUser(inboundTag, email, uuid, password, flow, method)
	return nil
}

// RemoveUserFromInbound removes a user entry from the inbound config JSON and calls xray API.
func (m *InboundManager) RemoveUserFromInbound(ctx context.Context, inboundTag, email string) error {
	inbound, err := m.inboundRepo.GetByTag(ctx, 1, inboundTag)
	if err != nil {
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		return nil
	}

	for _, key := range []string{"clients", "users"} {
		arr, ok := settings[key].([]interface{})
		if !ok {
			continue
		}
		var filtered []interface{}
		for _, u := range arr {
			user, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			if user["email"] != email {
				filtered = append(filtered, u)
			}
		}
		settings[key] = filtered
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	if err := m.inboundRepo.Update(ctx, inbound); err != nil {
		return err
	}

	// Call xray control API to remove user from running instance
	go m.callXrayRemoveUser(inboundTag, email)
	return nil
}

// PauseUser sets _paused=true and removes user from xray.
func (m *InboundManager) PauseUser(ctx context.Context, inboundID int64, email string) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	// Update database
	if err := m.setUserPaused(ctx, inboundID, email, true); err != nil {
		return err
	}

	// Call xray API to remove user from running instance
	go m.callXrayRemoveUser(inbound.Tag, email)
	return nil
}

// ResumeUser sets _paused=false and adds user back to xray.
func (m *InboundManager) ResumeUser(ctx context.Context, inboundID int64, email string) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	// Find user credentials from config
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}
	settings, _ := config["settings"].(map[string]interface{})
	var userEntry map[string]interface{}
	for _, key := range []string{"clients", "users"} {
		arr, ok := settings[key].([]interface{})
		if !ok {
			continue
		}
		for _, u := range arr {
			user, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			if user["email"] == email {
				userEntry = user
				break
			}
		}
		if userEntry != nil {
			break
		}
	}

	// Update database
	if err := m.setUserPaused(ctx, inboundID, email, false); err != nil {
		return err
	}

	// Call xray API to add user back
	if userEntry != nil {
		protocol := detectProtocol(inbound.Tag)
		uuid, _ := userEntry["id"].(string)
		password, _ := userEntry["password"].(string)
		auth, _ := userEntry["auth"].(string)
		flow, _ := userEntry["flow"].(string)
		method, _ := userEntry["method"].(string)
		if protocol == "hysteria" && auth != "" {
			password = auth
		}
		go m.callXrayAddUser(inbound.Tag, email, uuid, password, flow, method)
	}
	return nil
}

func (m *InboundManager) setUserPaused(ctx context.Context, inboundID int64, email string, paused bool) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		return nil
	}

	found := false
	for _, key := range []string{"clients", "users"} {
		arr, ok := settings[key].([]interface{})
		if !ok {
			continue
		}
		for _, u := range arr {
			user, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			if user["email"] == email {
				user["_paused"] = paused
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return nil
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	return m.inboundRepo.Update(ctx, inbound)
}

// callXrayAddUser calls xray control API to add a user.
func (m *InboundManager) callXrayAddUser(tag, email, uuid, password, flow, method string) {
	protocol := detectProtocol(tag)
	body := fmt.Sprintf(`{"email":"%s","id":"%s","password":"%s","flow":"%s","method":"%s"}`,
		email, uuid, password, flow, method)

	url := fmt.Sprintf("%s/api/v1/xray/control/inbound/%s/user", m.apiBaseURL, tag)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		log.Printf("xray add user: request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if token := m.generateInternalToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("xray add user %s to %s (%s): %v", email, tag, protocol, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("xray add user %s to %s: status %d", email, tag, resp.StatusCode)
	} else {
		log.Printf("xray add user %s to %s (%s): ok", email, tag, protocol)
	}
}

// callXrayRemoveUser calls xray control API to remove a user.
func (m *InboundManager) callXrayRemoveUser(tag, email string) {
	url := fmt.Sprintf("%s/api/v1/xray/control/inbound/%s/user/%s", m.apiBaseURL, tag, email)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("xray remove user: request error: %v", err)
		return
	}
	if token := m.generateInternalToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("xray remove user %s from %s: %v", email, tag, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("xray remove user %s from %s: status %d", email, tag, resp.StatusCode)
	} else {
		log.Printf("xray remove user %s from %s: ok", email, tag)
	}
}

// generateInternalToken generates a JWT token for internal API calls.
func (m *InboundManager) generateInternalToken() string {
	if m.jwtSecret == "" {
		return ""
	}
	j := jwt.New(m.jwtSecret, 24*time.Hour, 7*24*time.Hour)
	token, err := j.GenerateAccessToken(0, "internal", "super_admin")
	if err != nil {
		log.Printf("xray: failed to generate internal token: %v", err)
		return ""
	}
	return token
}

// detectProtocol extracts protocol from inbound tag.
func detectProtocol(tag string) string {
	for _, p := range []string{"vless", "vmess", "trojan", "hysteria", "shadowsocks", "ss-"} {
		if strings.Contains(tag, p) {
			if p == "ss-" {
				return "shadowsocks"
			}
			return p
		}
	}
	return "vless"
}
