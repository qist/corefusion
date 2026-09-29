package process

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const coreLogBufferMaxLines = 1000

// LogBroker captures log lines in a ring buffer
// and fans out to subscribers for real-time log streaming.
// This mirrors the proxy.LogBroker but lives in the process package
// to avoid import cycles.
type LogBroker struct {
	mu     sync.Mutex
	lines  []string
	subs   map[int]chan string
	nextID int
	stopCh chan struct{}
}

// NewLogBroker creates a new LogBroker.
func NewLogBroker() *LogBroker {
	return &LogBroker{
		lines:  make([]string, 0, coreLogBufferMaxLines),
		subs:   make(map[int]chan string),
		stopCh: make(chan struct{}),
	}
}

// append adds a line to the ring buffer and notifies subscribers.
func (b *LogBroker) append(line string) {
	b.mu.Lock()
	if len(b.lines) >= coreLogBufferMaxLines {
		copy(b.lines, b.lines[1:])
		b.lines[coreLogBufferMaxLines-1] = line
	} else {
		b.lines = append(b.lines, line)
	}
	for _, ch := range b.subs {
		select {
		case ch <- line:
		default:
		}
	}
	b.mu.Unlock()
}

// Snapshot returns the latest buffered lines.
func (b *LogBroker) Snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

// Subscribe returns a channel for live log lines and a cancel function.
func (b *LogBroker) Subscribe() (<-chan string, func()) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	ch := make(chan string, 200)
	b.subs[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if sub, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(sub)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// AttachCmd pipes cmd.Stdout and cmd.Stderr through the broker.
func (b *LogBroker) AttachCmd(cmd *exec.Cmd) {
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			b.append(time.Now().Format("2006/01/02 15:04:05 ") + scanner.Text())
		}
	}()
}

// TailFile starts tailing a log file and feeding lines to the broker.
func (b *LogBroker) TailFile(path string) {
	go func() {
		for {
			select {
			case <-b.stopCh:
				return
			default:
			}
			b.tailLoop(path)
			time.Sleep(1 * time.Second)
		}
	}()
}

func (b *LogBroker) tailLoop(path string) {
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		select {
		case <-b.stopCh:
			return
		case <-time.After(1 * time.Second):
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// tail -F 语义：不从文件头扫全量——大文件从末尾回退一个窗口开始，
	// 只把最近的行喂给 ring buffer（1000 行），跳过 seek 点所在的半行。
	const tailWindow = 256 * 1024
	var scanner *bufio.Scanner
	if info, err := f.Stat(); err == nil && info.Size() > tailWindow {
		f.Seek(-tailWindow, io.SeekEnd)
		br := bufio.NewReader(f)
		br.ReadString('\n')
		scanner = bufio.NewScanner(br)
	} else {
		scanner = bufio.NewScanner(f)
	}
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		b.append(scanner.Text())
	}

	// Get current position (file end at snapshot time)
	offset, _ := f.Seek(0, io.SeekEnd)

	for {
		select {
		case <-b.stopCh:
			return
		case <-time.After(300 * time.Millisecond):
		}

		info, err := os.Stat(path)
		if err != nil {
			return
		}

		if info.Size() < offset {
			offset = 0
			f.Close()
			f, err = os.Open(path)
			if err != nil {
				return
			}
			// 不用 defer：轮询循环内存活期不确定，defer 会累积到函数退出才关闭
		}

		if info.Size() > offset {
			f2, err := os.Open(path)
			if err != nil {
				continue
			}
			f2.Seek(offset, io.SeekStart)
			s := bufio.NewScanner(f2)
			s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for s.Scan() {
				b.append(s.Text())
			}
			offset, _ = f2.Seek(0, io.SeekCurrent)
			f2.Close()
		}
	}
}

// Stop stops the broker.
func (b *LogBroker) Stop() {
	select {
	case <-b.stopCh:
		// already closed
	default:
		close(b.stopCh)
	}
}
