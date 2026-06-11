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
func (s *Service) TailLog(ctx context.Context, logPath string, lastLines int) ([]string, error) {
	if lastLines <= 0 {
		lastLines = 500
	}

	resolved := s.resolveLogPath(logPath)
	f, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", resolved, err)
	}
	defer f.Close()

	lines, err := readLastLines(f, lastLines)
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// TailLogStream streams new log lines via SSE. It sends the last N lines first,
// then watches for new content and sends增量 lines.
func (s *Service) TailLogStream(ctx context.Context, logPath string, lastLines int, ch chan<- string) error {
	defer close(ch)

	if lastLines <= 0 {
		lastLines = 200
	}

	resolved := s.resolveLogPath(logPath)

	// Send initial lines
	f, err := os.Open(resolved)
	if err != nil {
		return fmt.Errorf("open %s: %w", resolved, err)
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

	// Watch for new content using polling
	// Start from the current file size
	info, err := os.Stat(resolved)
	if err != nil {
		return nil // file might have been rotated
	}
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
				continue // file might be rotating
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
