package logmanage

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogWriter manages log output mode using symlinks (like Docker nginx image).
// stdout mode: logs/access.log -> /dev/stdout
// file mode:   logs/access.log -> logs/nginx/access.current.log (real file)
type LogWriter struct {
	mu         sync.Mutex
	proxyType  string
	dataDir    string
	retainDays int
	compress   bool
	maxSizeMB  int
	active     bool
	stopCh     chan struct{}
	ticker     *time.Ticker
}

// LogWriterConfig configures a LogWriter.
type LogWriterConfig struct {
	ProxyType  string `json:"proxy_type"`
	Mode       string `json:"mode"` // "stdout" or "file"
	RetainDays int    `json:"retain_days"`
	Compress   bool   `json:"compress"`
	MaxSizeMB  int    `json:"max_size_mb"`
}

// NewLogWriter creates a new LogWriter.
func NewLogWriter(cfg LogWriterConfig, dataDir string) *LogWriter {
	if cfg.RetainDays <= 0 {
		cfg.RetainDays = 30
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 100
	}
	return &LogWriter{
		proxyType:  cfg.ProxyType,
		dataDir:    dataDir,
		retainDays: cfg.RetainDays,
		compress:   cfg.Compress,
		maxSizeMB:  cfg.MaxSizeMB,
		stopCh:     make(chan struct{}),
	}
}

// SetMode switches the log output mode (stdout or file).
func (w *LogWriter) SetMode(mode string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logDir := filepath.Join(w.dataDir, "logs")
	os.MkdirAll(logDir, 0755)

	switch mode {
	case "stdout":
		// Symlink logs/access.log -> /dev/stdout
		accessLog := filepath.Join(logDir, "access.log")
		w.symlink(accessLog, "/dev/stdout")
		log.Printf("logwriter %s: mode=stdout, symlinked to /dev/stdout", w.proxyType)

	case "file":
		// Create proxy-specific directory for rotation
		proxyDir := filepath.Join(logDir, w.proxyType)
		os.MkdirAll(proxyDir, 0755)

		accessCurrent := filepath.Join(proxyDir, "access.current.log")
		if _, err := os.Stat(accessCurrent); os.IsNotExist(err) {
			os.WriteFile(accessCurrent, []byte{}, 0644)
		}

		// Only nginx creates symlinks (caddy writes to stdout)
		if w.proxyType == "nginx" {
			accessLog := filepath.Join(logDir, "access.log")
			relPath, _ := filepath.Rel(logDir, accessCurrent)
			w.symlink(accessLog, relPath)
			log.Printf("logwriter %s: mode=file, symlinked to %s", w.proxyType, relPath)
		}

		// Start rotation checker
		if !w.active {
			w.active = true
			w.ticker = time.NewTicker(1 * time.Minute)
			go w.checkLoop()
		}

	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}

	return nil
}

// Stop stops the rotation checker.
func (w *LogWriter) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.active {
		return
	}
	w.active = false
	close(w.stopCh)
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

// IsActive returns whether the writer is active (file mode with rotation).
func (w *LogWriter) IsActive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

// symlink creates a symlink, removing existing file/symlink first.
func (w *LogWriter) symlink(path, target string) {
	os.Remove(path)
	os.Symlink(target, path)
}

// GetLogDir returns the proxy-specific log directory.
func (w *LogWriter) GetLogDir() string {
	return filepath.Join(w.dataDir, "logs", w.proxyType)
}

// checkLoop periodically checks for rotation.
func (w *LogWriter) checkLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ticker.C:
			w.checkAndRotate()
		}
	}
}

// checkAndRotate checks if rotation is needed and performs it.
func (w *LogWriter) checkAndRotate() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.active {
		return
	}

	currentPath := filepath.Join(w.dataDir, "logs", w.proxyType, "access.current.log")
	info, err := os.Stat(currentPath)
	if err != nil {
		return
	}

	// Rotate if size exceeded
	if w.maxSizeMB > 0 && info.Size() >= int64(w.maxSizeMB)*1024*1024 {
		w.rotate(currentPath)
		return
	}

	// Rotate daily
	if info.ModTime().Day() != time.Now().Day() {
		w.rotate(currentPath)
		return
	}

	w.cleanExpired()
}

// rotate performs log rotation.
func (w *LogWriter) rotate(currentPath string) {
	dateSuffix := time.Now().Format("20060102")
	dir := filepath.Dir(currentPath)
	rotatedPath := filepath.Join(dir, fmt.Sprintf("access.%s.log", dateSuffix))

	// Handle duplicates
	if _, err := os.Stat(rotatedPath); err == nil {
		for i := 2; i < 100; i++ {
			rotatedPath = filepath.Join(dir, fmt.Sprintf("access.%s.%d.log", dateSuffix, i))
			if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if err := os.Rename(currentPath, rotatedPath); err != nil {
		log.Printf("logwriter %s: rotate error: %v", w.proxyType, err)
		return
	}

	os.WriteFile(currentPath, []byte{}, 0644)

	if w.compress {
		go w.compressFile(rotatedPath)
	}

	log.Printf("logwriter %s: rotated to %s", w.proxyType, rotatedPath)
}

// compressFile compresses a file using gzip.
func (w *LogWriter) compressFile(path string) {
	src, err := os.Open(path)
	if err != nil {
		return
	}
	defer src.Close()

	dstPath := path + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		return
	}

	gw := gzip.NewWriter(dst)
	if _, err := io.Copy(gw, src); err != nil {
		gw.Close()
		dst.Close()
		os.Remove(dstPath)
		return
	}
	gw.Close()
	dst.Close()
	os.Remove(path)
}

// cleanExpired removes rotated files older than retainDays.
func (w *LogWriter) cleanExpired() {
	dir := filepath.Join(w.dataDir, "logs", w.proxyType)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -w.retainDays)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "access.") || strings.HasSuffix(name, "current.log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, name))
			log.Printf("logwriter %s: cleaned %s", w.proxyType, name)
		}
	}
}

// ListLogFiles returns log files in the proxy directory.
func (w *LogWriter) ListLogFiles() []LogFileInfo {
	dir := filepath.Join(w.dataDir, "logs", w.proxyType)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []LogFileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	return files
}

// LogFileInfo contains information about a log file.
type LogFileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}
