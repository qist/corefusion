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

// LogWriter manages log output mode and file rotation.
// stdout mode: captures from LogBroker, writes to nginx-stdout.log with rotation
// file mode:   manages rotation of proxy log files
type LogWriter struct {
	mu         sync.Mutex
	proxyType  string
	dataDir    string
	filePath   string // configurable stdout capture file path (relative to dataDir)
	retainDays int
	compress   bool
	maxSizeMB  int
	active     bool
	paused     bool // true = subscription active but not writing
	stopCh     chan struct{}
	ticker     *time.Ticker
	logFile    *os.File // open file for writing in stdout mode
	cancelSub  func()   // cancel LogBroker subscription
}

// LogWriterConfig configures a LogWriter.
type LogWriterConfig struct {
	ProxyType  string `json:"proxy_type"`
	Mode       string `json:"mode"`      // "stdout", "file", or "off"
	FilePath   string `json:"file_path"` // stdout capture file path (relative to dataDir)
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
	filePath := cfg.FilePath
	if filePath == "" {
		filePath = fmt.Sprintf("logs/%s-stdout.log", cfg.ProxyType)
	}
	return &LogWriter{
		proxyType:  cfg.ProxyType,
		dataDir:    dataDir,
		filePath:   filePath,
		retainDays: cfg.RetainDays,
		compress:   cfg.Compress,
		maxSizeMB:  cfg.MaxSizeMB,
		stopCh:     make(chan struct{}),
	}
}

// SetMode switches the log output mode (stdout, file, or off).
func (w *LogWriter) SetMode(mode string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	logDir := filepath.Join(w.dataDir, "logs")
	os.MkdirAll(logDir, 0755)

	switch mode {
	case "stdout":
		// stdout mode: open the capture file for writing
		logPath := w.resolveFilePath()
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open %s: %w", logPath, err)
		}
		w.logFile = f
		if !w.active {
			w.active = true
			w.ticker = time.NewTicker(1 * time.Minute)
			go w.checkLoop()
		}
		log.Printf("logwriter %s: mode=stdout, writing to %s", w.proxyType, logPath)

	case "file":
		// File mode: process writes to its configured log paths
		// We manage rotation of those files
		if !w.active {
			w.active = true
			w.ticker = time.NewTicker(1 * time.Minute)
			go w.checkLoop()
		}
		log.Printf("logwriter %s: mode=file", w.proxyType)

	case "off":
		log.Printf("logwriter %s: mode=off", w.proxyType)

	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}

	return nil
}

// Stop stops the rotation checker and closes the log file.
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
	if w.cancelSub != nil {
		w.cancelSub()
		w.cancelSub = nil
	}
	if w.logFile != nil {
		w.logFile.Close()
		w.logFile = nil
	}
}

// SetCancel sets the cancel function for the LogBroker subscription.
func (w *LogWriter) SetCancel(cancel func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancelSub = cancel
}

// SetPaused pauses or resumes writing (subscription stays active).
func (w *LogWriter) SetPaused(paused bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = paused
	if !paused && w.logFile == nil {
		// Reopen file if needed
		logPath := w.resolveFilePath()
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			w.logFile = f
		}
	}
}

// WriteLine writes a log line to the output file (for stdout mode).
func (w *LogWriter) WriteLine(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.paused || w.logFile == nil {
		return
	}
	w.logFile.WriteString(line + "\n")
}

// IsActive returns whether the writer is active (not stopped).
func (w *LogWriter) IsActive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

// IsPaused returns whether writing is paused.
func (w *LogWriter) IsPaused() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.paused
}

// GetLogDir returns the log directory.
func (w *LogWriter) GetLogDir() string {
	return filepath.Join(w.dataDir, "logs")
}

// GetFilePath returns the configured stdout capture file path (relative to dataDir).
func (w *LogWriter) GetFilePath() string {
	return w.filePath
}

// resolveFilePath returns the absolute path of the stdout capture file.
func (w *LogWriter) resolveFilePath() string {
	if filepath.IsAbs(w.filePath) {
		return w.filePath
	}
	return filepath.Join(w.dataDir, w.filePath)
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
// Rotation only happens when the file size reaches maxSizeMB.
// Expired file cleanup always runs regardless of rotation.
func (w *LogWriter) checkAndRotate() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.active {
		return
	}

	// Always clean expired files
	w.cleanExpired()

	// Determine which file to check
	stdoutPath := w.resolveFilePath()

	var currentPath string
	if _, err := os.Stat(stdoutPath); err == nil {
		currentPath = stdoutPath
	} else {
		return
	}

	info, err := os.Stat(currentPath)
	if err != nil {
		return
	}

	// Only rotate when file size reaches the threshold.
	// This prevents splitting small log files that haven't grown enough.
	if w.maxSizeMB > 0 && info.Size() >= int64(w.maxSizeMB)*1024*1024 {
		w.rotate(currentPath)
		return
	}
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

	// Reopen the file handle: after the rename the old FD points at the
	// rotated file, and without reopening, every subsequent line would be
	// appended to the ARCHIVE instead of the fresh current file.
	if w.logFile != nil {
		w.logFile.Close()
		w.logFile = nil
	}
	f, err := os.OpenFile(currentPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("logwriter %s: reopen after rotate failed: %v", w.proxyType, err)
	} else {
		w.logFile = f
	}

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
	logDir := filepath.Join(w.dataDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	// Derive the file base name (without extension) from the configured file path
	baseName := strings.TrimSuffix(filepath.Base(w.filePath), filepath.Ext(w.filePath))
	prefix := baseName + "."
	currentName := filepath.Base(w.filePath)
	cutoff := time.Now().AddDate(0, 0, -w.retainDays)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		// Skip the current log file
		if name == currentName {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, name))
			log.Printf("logwriter %s: cleaned %s", w.proxyType, name)
		}
	}
}

// ListLogFiles returns log files for this writer.
func (w *LogWriter) ListLogFiles() []LogFileInfo {
	logDir := filepath.Join(w.dataDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	// Derive the file base name (without extension) from the configured file path
	baseName := strings.TrimSuffix(filepath.Base(w.filePath), filepath.Ext(w.filePath))
	var files []LogFileInfo
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, baseName) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{Name: name, Size: info.Size(), ModTime: info.ModTime()})
	}
	return files
}

// LogFileInfo contains information about a log file.
type LogFileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}
