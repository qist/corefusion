package logmanage

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// TailLog reads the last N lines from a log file.
// If the file doesn't exist, falls back to proxy stdout broker.
func (s *Service) TailLog(ctx context.Context, logPath string, lastLines int) ([]string, error) {
	if lastLines <= 0 {
		lastLines = 500
	}

	resolved := s.resolveLogPath(logPath)

	// If file exists, read from it
	if _, err := os.Stat(resolved); err == nil {
		f, err := os.Open(resolved)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", resolved, err)
		}
		defer f.Close()
		return readLastLines(f, lastLines)
	}

	// File doesn't exist - try proxy stdout broker
	if s.proxyBroker != nil {
		for _, pType := range []string{"nginx", "caddy"} {
			broker := s.proxyBroker.GetLogBroker(pType)
			if broker != nil {
				lines := broker.Snapshot()
				if len(lines) > 0 {
					if len(lines) > lastLines {
						lines = lines[len(lines)-lastLines:]
					}
					return lines, nil
				}
			}
		}
	}

	return []string{
		fmt.Sprintf("[日志文件不存在] %s", resolved),
		"[提示] 代理可能正在输出到标准输出(stdout)",
		"[提示] 请在日志管理中心点击「接管捕获」，或在代理管理页面切换为「输出到文件」",
	}, nil
}

// TailLogStream streams new log lines via SSE.
// For file-based logs: sends initial lines then polls for new content.
// For stdout logs: subscribes to the broker for real-time streaming.
func (s *Service) TailLogStream(ctx context.Context, logPath string, lastLines int, ch chan<- string) error {
	defer close(ch)

	if lastLines <= 0 {
		lastLines = 200
	}

	resolved := s.resolveLogPath(logPath)

	// Check if file exists
	if _, err := os.Stat(resolved); err == nil {
		// File-based: send initial lines then poll
		return s.streamFromFile(ctx, resolved, lastLines, ch)
	}

	// File doesn't exist - try broker for real-time streaming
	if s.proxyBroker != nil {
		for _, pType := range []string{"nginx", "caddy"} {
			broker := s.proxyBroker.GetLogBroker(pType)
			if broker != nil {
				return s.streamFromBroker(ctx, broker, lastLines, ch)
			}
		}
	}

	// No source available
	ch <- "[无可用日志源]"
	return nil
}

// streamFromFile streams from a log file using polling.
func (s *Service) streamFromFile(ctx context.Context, resolved string, lastLines int, ch chan<- string) error {
	f, err := os.Open(resolved)
	if err != nil {
		return err
	}
	lines, err := readLastLines(f, lastLines)
	f.Close()
	if err != nil {
		return err
	}
	for _, line := range lines {
		select {
		case <-ctx.Done():
			return nil
		case ch <- line:
		}
	}

	info, _ := os.Stat(resolved)
	lastSize := info.Size()
	lastMod := info.ModTime()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			newLines, newSize, newMod, err := s.readNewContent(resolved, lastSize, lastMod)
			if err != nil {
				continue
			}
			if len(newLines) > 0 {
				lastSize = newSize
				lastMod = newMod
				for _, line := range newLines {
					select {
					case <-ctx.Done():
						return nil
					case ch <- line:
					}
				}
			}
		}
	}
}

// streamFromBroker streams from a proxy log broker using pub/sub.
func (s *Service) streamFromBroker(ctx context.Context, broker LogBrokerReader, lastLines int, ch chan<- string) error {
	// Send snapshot first
	snapshot := broker.Snapshot()
	if len(snapshot) > lastLines {
		snapshot = snapshot[len(snapshot)-lastLines:]
	}
	for _, line := range snapshot {
		select {
		case <-ctx.Done():
			return nil
		case ch <- line:
		}
	}

	// Subscribe for live updates
	liveCh, cancel := broker.Subscribe()
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-liveCh:
			if !ok {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case ch <- line:
			}
		}
	}
}

// readNewContent reads new content appended to a file since the given size.
func (s *Service) readNewContent(path string, lastSize int64, lastMod time.Time) ([]string, int64, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, lastMod, err
	}

	// File was truncated or rotated (size decreased)
	if info.Size() < lastSize {
		lastSize = 0
	}

	// No new content
	if info.Size() == lastSize {
		return nil, lastSize, info.ModTime(), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, lastMod, err
	}
	defer f.Close()

	if _, err := f.Seek(lastSize, io.SeekStart); err != nil {
		return nil, 0, lastMod, err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	newSize := lastSize + int64(0)
	if fi, err := f.Stat(); err == nil {
		newSize = fi.Size()
	}

	return lines, newSize, info.ModTime(), nil
}

// readLastLines reads the last N lines from a file efficiently.
func readLastLines(f *os.File, n int) ([]string, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return []string{}, nil
	}

	// Read backwards to find N newlines
	bufSize := int64(64 * 1024) // 64KB chunks
	var lines []string
	var offset int64 = fileSize
	var leftover []byte

	for len(lines) < n+1 && offset > 0 {
		readSize := bufSize
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, offset); err != nil && err != io.EOF {
			return nil, err
		}

		// Prepend chunk to leftover
		data := append(chunk, leftover...)

		// Split into lines
		for i := len(data) - 1; i >= 0; i-- {
			if data[i] == '\n' {
				line := string(data[i+1:])
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
				if line != "" {
					lines = append([]string{line}, lines...)
				}
				if len(lines) >= n {
					break
				}
				data = data[:i]
			}
		}
		leftover = data

		if len(lines) >= n {
			break
		}
	}

	// Handle remaining data (beginning of file)
	if len(leftover) > 0 && len(lines) < n {
		line := string(leftover)
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line != "" {
			lines = append([]string{line}, lines...)
		}
	}

	// Trim to exactly N lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	log.Printf("logmanage tailer: read %d lines from file", len(lines))
	return lines, nil
}
