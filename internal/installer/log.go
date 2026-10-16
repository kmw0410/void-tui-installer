package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
)

// diagnosticLog retains bounded, sanitized diagnostics after terminal teardown.
type diagnosticLog struct {
	mu   sync.Mutex
	file *os.File
	size int64
	err  error
}

func newDiagnosticLog() (*diagnosticLog, error) {
	f, err := os.CreateTemp("", "void-installer-*.log")
	if err != nil {
		return nil, err
	}
	return &diagnosticLog{file: f}, nil
}

func (l *diagnosticLog) write(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		l.err = err
		return
	}
	if l.size+int64(len(data))+1 > 8<<20 {
		tail := make([]byte, 4<<20)
		n, err := l.file.ReadAt(tail, l.size-int64(len(tail)))
		if err != nil {
			l.err = err
			return
		}
		tail = tail[:n]
		if i := bytes.IndexByte(tail, '\n'); i >= 0 {
			tail = tail[i+1:]
		}
		if err = l.file.Truncate(0); err != nil {
			l.err = err
			return
		}
		if _, err = l.file.Seek(0, 0); err != nil {
			l.err = err
			return
		}
		marker := []byte("{\"version\":1,\"type\":\"log\",\"level\":\"warning\",\"message\":\"Earlier diagnostics rotated to bound log size\"}\n")
		n, err = l.file.Write(append(marker, tail...))
		if err != nil {
			l.err = err
			return
		}
		l.size = int64(n)
	}
	n, err := l.file.Write(append(data, '\n'))
	l.size += int64(n)
	l.err = err
}

func (l *diagnosticLog) close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.file.Sync(); l.err == nil {
		l.err = err
	}
	if err := l.file.Close(); l.err == nil {
		l.err = err
	}
	return l.err
}
