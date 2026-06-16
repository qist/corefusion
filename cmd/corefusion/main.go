package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/router"
	"corefusion/internal/auth/jwt"
	"corefusion/internal/migration"
	authService "corefusion/internal/service/auth"
	sysService "corefusion/internal/service/system"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		runServer(os.Args[2:])
	case "client":
		runClient(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "version":
		fmt.Printf("corefusion %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`CoreFusion - Multi-protocol proxy management panel

Usage:
  corefusion <command> [options]

Commands:
  server    Start the server management panel
  client    Start the client mode
  migrate   Run database migrations
  version   Print version
  help      Show this help

`)
}

// loadEnv reads the .env file and returns a map of key-value pairs.
func loadEnv() map[string]string {
	env := make(map[string]string)
	f, err := os.Open(".env")
	if err != nil {
		return env
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return env
}

// saveEnvValue updates or adds a key-value pair in the .env file.
func saveEnvValue(key, value string) {
	lines := []string{}
	found := false

	if data, err := os.ReadFile(".env"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
					lines = append(lines, key+"="+value)
					found = true
					continue
				}
			}
			lines = append(lines, line)
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func runServer(args []string) {
	// Record panel start time
	sysService.InitStartTime()

	// Phase 1: Read .env to get data directory
	env := loadEnv()
	dataDir := env["SAFE_PATHS"]
	if dataDir == "" {
		dataDir = "data"
	}

	// Ensure data directory structure
	initDataDir(dataDir)

	// Phase 2: Initialize database
	dbPath := filepath.Join(dataDir, "sqlite", "app.db")
	dbCfg := db.DefaultConfig()
	dbCfg.Path = dbPath
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Phase 3: Read panel settings from database
	settingRepo := repository.NewSystemSettingRepository(db.DB)
	settings := readSettings(settingRepo)

	// Sync data_dir in DB with .env (env is source of truth)
	if dbDataDir := settings["data_dir"]; dbDataDir != dataDir {
		settingRepo.Set(nil, "panel.data_dir", dataDir)
		settings["data_dir"] = dataDir
	}

	log.Printf("Data directory: %s", dataDir)

	// Phase 4: Setup JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "corefusion-dev-secret"
	}
	jwtHandler := jwt.New(jwtSecret, 0, 0)
	middleware.SetJWT(jwtHandler)

	// Create initial admin
	userRepo := repository.NewUserRepository(db.DB)
	authSvc := authService.NewService(userRepo, jwtSecret)
	if err := authSvc.CreateInitialAdmin(context.Background(), "admin", "admin123"); err != nil {
		log.Printf("initial admin: %v", err)
	}

	// Phase 5: Create router
	r := router.NewWithFullConfig(db.DB, jwtSecret, settings["web_path"], dataDir, Version)

	// Phase 6: Build listen address
	listenAddr := ":8080"
	if addr := settings["listen_addr"]; addr != "" {
		listenAddr = addr + ":"
	}
	if port := settings["listen_port"]; port != "" {
		if listenAddr[len(listenAddr)-1] == ':' {
			listenAddr = listenAddr + port
		}
	}
	if envPort := os.Getenv("PORT"); envPort != "" {
		listenAddr = ":" + envPort
	}

	// Phase 7: TLS configuration
	tlsEnabled := settings["tls_enabled"] == "true"
	tlsCertPath := settings["tls_cert_path"]
	tlsKeyPath := settings["tls_key_path"]
	mtlsEnabled := settings["mtls_enabled"] == "true"
	mtlsCAPath := settings["mtls_ca_path"]

	if tlsEnabled && (tlsCertPath == "" || tlsKeyPath == "") {
		log.Println("WARNING: TLS enabled but cert/key paths not set, falling back to HTTP")
		tlsEnabled = false
	}

	// Phase 8: Start server with graceful shutdown
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("shutting down...")
		router.Shutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	if tlsEnabled {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

		if mtlsEnabled && mtlsCAPath != "" {
			caCert, err := os.ReadFile(mtlsCAPath)
			if err != nil {
				log.Printf("WARNING: failed to read mTLS CA: %v, skipping mTLS", err)
			} else {
				pool := x509.NewCertPool()
				if pool.AppendCertsFromPEM(caCert) {
					tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
					tlsCfg.ClientCAs = pool
					log.Printf("mTLS enabled, CA: %s", mtlsCAPath)
				} else {
					log.Println("WARNING: failed to parse mTLS CA cert")
				}
			}
		}

		srv.TLSConfig = tlsCfg
		log.Printf("CoreFusion %s starting on %s (HTTPS)", Version, listenAddr)
		if err := srv.ListenAndServeTLS(tlsCertPath, tlsKeyPath); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	} else {
		log.Printf("CoreFusion %s starting on %s (HTTP)", Version, listenAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}
}

// initDataDir creates the data directory structure.
func initDataDir(dataDir string) {
	dirs := []string{
		filepath.Join(dataDir, "sqlite"),
		filepath.Join(dataDir, "certs"),
		filepath.Join(dataDir, "certs", "mtls"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "backup"),
		filepath.Join(dataDir, "runtime"),
		filepath.Join(dataDir, "rules"),
		filepath.Join(dataDir, "rules_tmp"),
		filepath.Join(dataDir, "binaries", "active"),
		filepath.Join(dataDir, "binaries", "backup"),
		filepath.Join(dataDir, "cache"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0750)
	}
}

// readSettings reads all panel settings from database.
func readSettings(repo *repository.SystemSettingRepository) map[string]string {
	keys := []string{
		"panel.listen_addr", "panel.listen_port",
		"panel.tls_enabled", "panel.tls_cert_path", "panel.tls_key_path",
		"panel.mtls_enabled", "panel.mtls_ca_path",
		"panel.web_path", "panel.timezone", "panel.data_dir",
	}
	settings := make(map[string]string)
	for _, key := range keys {
		if v, err := repo.GetByKey(nil, key); err == nil && v != "" {
			settings[key[len("panel."):]] = v
		}
	}
	return settings
}

func runClient(args []string) {
	fmt.Println("client mode not yet implemented")
}

func runMigrate(args []string) {
	env := loadEnv()
	dataDir := env["SAFE_PATHS"]
	if dataDir == "" {
		dataDir = "data"
	}

	dbCfg := db.DefaultConfig()
	dbCfg.Path = filepath.Join(dataDir, "sqlite", "app.db")
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.Close()

	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed successfully")
}
