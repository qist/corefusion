package cert

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Port80Strategy defines how to handle port 80 for ACME HTTP-01 challenges.
type Port80Strategy int

const (
	// Port80StrategyDirect means port 80 is free, start a temporary server directly.
	Port80StrategyDirect Port80Strategy = iota
	// Port80StrategyProxy means the panel's proxy (nginx/caddy) is on port 80,
	// so we use the Gin router to serve challenges.
	Port80StrategyProxy
	// Port80StrategyConflict means port 80 is occupied by an external service.
	Port80StrategyConflict
)

// Port80Status contains information about port 80 status.
type Port80Status struct {
	Strategy  Port80Strategy
	InUse     bool
	Process   string // Process info if port 80 is in use
	IsManaged bool   // True if managed by our panel's proxy
}

// HTTP01Provider implements the challenge.Provider interface for HTTP-01 challenges.
// It supports three modes:
// 1. Direct: Start a temporary HTTP server on port 80 (when port 80 is free)
// 2. Proxy: Serve challenges through the Gin router (when panel's proxy manages port 80)
// 3. Conflict: Return error if port 80 is occupied by external service
type HTTP01Provider struct {
	mu         sync.RWMutex
	challenges map[string]string // token -> keyAuth

	// Strategy for handling port 80
	strategy Port80Strategy

	// Temporary HTTP server for direct mode
	tempServer *http.Server
	tempPort   string

	// Proxy mode: the Gin handler serves challenges
	ginHandler gin.HandlerFunc

	// Port checking
	panelPort string // The panel's own port (e.g., "8080")
}

// NewHTTP01Provider creates a new HTTP-01 challenge provider.
func NewHTTP01Provider() *HTTP01Provider {
	return &HTTP01Provider{
		challenges: make(map[string]string),
		tempPort:   "80",
		panelPort:  "8080",
	}
}

// SetPanelPort sets the panel's port for proxy mode detection.
func (p *HTTP01Provider) SetPanelPort(port string) {
	p.panelPort = port
}

// CheckPort80Status checks the status of port 80 and determines the strategy.
func (p *HTTP01Provider) CheckPort80Status() (*Port80Status, error) {
	status := &Port80Status{}

	// Try to listen on port 80
	ln, err := net.Listen("tcp", ":80")
	if err != nil {
		// Port 80 is in use
		status.InUse = true
		status.Process = findProcessOnPort("80")

		// Check if it's our panel's proxy (nginx/caddy managed by us)
		if p.isManagedProxy("80") {
			status.Strategy = Port80StrategyProxy
			status.IsManaged = true
			log.Printf("[ACME-HTTP01] Port 80 is managed by panel proxy, using proxy mode")
		} else {
			status.Strategy = Port80StrategyConflict
			log.Printf("[ACME-HTTP01] Port 80 is occupied by external service: %s", status.Process)
		}
		return status, nil
	}

	// Port 80 is free
	ln.Close()
	status.Strategy = Port80StrategyDirect
	log.Printf("[ACME-HTTP01] Port 80 is free, using direct mode")
	return status, nil
}

// isManagedProxy checks if port 80 is managed by our panel's nginx/caddy.
func (p *HTTP01Provider) isManagedProxy(port string) bool {
	// Check if nginx or caddy processes are running and managed by us
	// Look for our data directory in the process command line
	nginxManaged := isProcessManagedByUs("nginx")
	caddyManaged := isProcessManagedByUs("caddy")
	return nginxManaged || caddyManaged
}

// findProcessOnPort finds the process using a specific port.
func findProcessOnPort(port string) string {
	// Use lsof to find the process
	cmd := exec.Command("lsof", "-i", ":"+port, "-t")
	out, err := cmd.Output()
	if err != nil {
		return "unknown process"
	}
	pid := strings.TrimSpace(string(out))
	if pid == "" {
		return "unknown process"
	}
	// Get process name
	cmd2 := exec.Command("ps", "-p", pid, "-o", "comm=")
	out2, err := cmd2.Output()
	if err != nil {
		return "PID " + pid
	}
	return strings.TrimSpace(string(out2)) + " (PID " + pid + ")"
}

// isProcessManagedByUs checks if a process is managed by our panel.
func isProcessManagedByUs(processName string) bool {
	// Check if the process is running and its command line contains our data directory
	cmd := exec.Command("pgrep", "-f", processName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	pids := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, pid := range pids {
		if pid == "" {
			continue
		}
		// Read the command line of the process
		cmdlinePath := fmt.Sprintf("/proc/%s/cmdline", pid)
		cmdline, err := os.ReadFile(cmdlinePath)
		if err != nil {
			continue
		}
		// Check if it's managed by our panel (contains our data directory)
		if strings.Contains(string(cmdline), "data/proxy") || strings.Contains(string(cmdline), "corefusion") {
			return true
		}
	}
	return false
}

// Present stores the challenge token and key authorization for serving.
// Called by lego when an HTTP-01 challenge needs to be presented.
func (p *HTTP01Provider) Present(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.challenges[token] = keyAuth
	log.Printf("[ACME-HTTP01] Presenting challenge for domain=%s token=%s", domain, token)

	// If using direct mode, ensure the temporary server is running
	if p.strategy == Port80StrategyDirect {
		if err := p.ensureTempServer(); err != nil {
			return fmt.Errorf("start temporary HTTP server: %w", err)
		}
	}

	return nil
}

// CleanUp removes the challenge token after validation.
// Called by lego when the challenge is complete (success or failure).
func (p *HTTP01Provider) CleanUp(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.challenges, token)
	log.Printf("[ACME-HTTP01] Cleaned up challenge for domain=%s token=%s", domain, token)

	// If no more challenges, stop the temporary server
	if p.strategy == Port80StrategyDirect && len(p.challenges) == 0 {
		p.stopTempServer()
	}

	return nil
}

// ensureTempServer starts the temporary HTTP server if not already running.
func (p *HTTP01Provider) ensureTempServer() error {
	if p.tempServer != nil {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/", p.handleChallenge)

	p.tempServer = &http.Server{
		Addr:    ":80",
		Handler: mux,
	}

	go func() {
		log.Printf("[ACME-HTTP01] Starting temporary HTTP server on port %s", p.tempPort)
		if err := p.tempServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ACME-HTTP01] Temporary server error: %v", err)
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// stopTempServer stops the temporary HTTP server.
func (p *HTTP01Provider) stopTempServer() {
	if p.tempServer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("[ACME-HTTP01] Stopping temporary HTTP server")
	p.tempServer.Shutdown(ctx)
	p.tempServer = nil
}

// handleChallenge handles ACME HTTP-01 challenge requests for the temporary server.
func (p *HTTP01Provider) handleChallenge(w http.ResponseWriter, r *http.Request) {
	// Extract token from path: /.well-known/acme-challenge/<token>
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	token := parts[len(parts)-1]

	if token == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	keyAuth, ok := p.challenges[token]
	p.mu.RUnlock()

	if !ok {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	// Return the key authorization with correct content type
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(keyAuth))
}

// Handler returns a Gin handler that serves ACME HTTP-01 challenge responses.
// This should be registered at /.well-known/acme-challenge/:token
// Used in proxy mode when the panel's proxy manages port 80.
func (p *HTTP01Provider) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if token == "" {
			// Also check the full path for compatibility
			path := c.Request.URL.Path
			parts := strings.Split(path, "/")
			if len(parts) > 0 {
				token = parts[len(parts)-1]
			}
		}

		if token == "" {
			c.String(http.StatusBadRequest, "")
			return
		}

		p.mu.RLock()
		keyAuth, ok := p.challenges[token]
		p.mu.RUnlock()

		if !ok {
			c.String(http.StatusNotFound, "")
			return
		}

		// Return the key authorization with correct content type
		// acme.sh and the ACME spec expect text/plain
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(keyAuth))
	}
}

// HasActiveChallenges returns true if there are active challenges being served.
func (p *HTTP01Provider) HasActiveChallenges() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.challenges) > 0
}

// GetStrategy returns the current port 80 strategy.
func (p *HTTP01Provider) GetStrategy() Port80Strategy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.strategy
}

// SetStrategy sets the port 80 strategy.
func (p *HTTP01Provider) SetStrategy(strategy Port80Strategy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.strategy = strategy
}

// GetChallengePath returns the expected challenge path for a given token.
// Useful for logging and debugging.
func GetChallengePath(token string) string {
	return fmt.Sprintf("/.well-known/acme-challenge/%s", token)
}
