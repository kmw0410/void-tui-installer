package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"void-installer/internal/validation"
	"void-installer/scripts"
)

type Session struct {
	Events  <-chan Event
	Done    <-chan error
	LogPath string
	cancel  context.CancelFunc
	stopped chan struct{}
	result  error
}

func (s *Session) Cancel()     { s.cancel() }
func (s *Session) Wait() error { <-s.stopped; return s.result }

func extractEngine() (string, error) {
	dir, err := os.MkdirTemp("", "void-installer-engine-")
	if err != nil {
		return "", fmt.Errorf("Cannot create private engine directory")
	}
	err = fs.WalkDir(scripts.FS, ".", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(dir, path)
		if d.IsDir() {
			return os.Mkdir(target, 0700)
		}
		data, e := scripts.FS.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("Cannot extract installation engine")
	}
	return dir, nil
}
func Preflight() error { return PreflightContext(context.Background()) }
func PreflightContext(parent context.Context) error {
	return CheckMirrorContext(parent, DefaultMirror)
}

// PrepareLiveContext installs missing installation tools only in a validated Void Live session.
func PrepareLiveContext(parent context.Context) error { return PrepareLiveProgressContext(parent, nil) }

// PrepareLiveProgressContext reports only trusted preparation stage markers.
func PrepareLiveProgressContext(parent context.Context, report func(string, string)) error {
	dir, err := extractEngine()
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return preflightScriptProgress(parent, filepath.Join(dir, "install.sh"), report, "--prepare-live")
}

func CheckLocalContext(parent context.Context) error { return checkMode(parent, "--check-local") }
func CheckMirrorContext(parent context.Context, mirror string) error {
	root, err := NormalizeMirror(mirror)
	if err != nil {
		return err
	}
	return checkMode(parent, "--check", root)
}
func checkMode(parent context.Context, args ...string) error {
	dir, err := extractEngine()
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return preflightScript(parent, filepath.Join(dir, "install.sh"), args...)
}

func preflightScript(parent context.Context, script string, args ...string) error {
	return preflightScriptProgress(parent, script, nil, args...)
}
func preflightScriptProgress(parent context.Context, script string, report func(string, string), args ...string) error {
	log, err := os.CreateTemp("", "void-installer-preflight-*.log")
	if err != nil {
		return fmt.Errorf("Cannot create a private environment-check log")
	}
	defer log.Close()
	ctx, cancel := context.WithTimeout(parent, 8*time.Minute)
	defer cancel()
	if len(args) == 0 {
		args = []string{"--check"}
	}
	cmd := exec.CommandContext(ctx, "/bin/bash", append([]string{script}, args...)...)
	cmd.Env = safeEnv()
	writer := &preparationWriter{log: log, report: report}
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 3 * time.Second
	err = cmd.Run()
	if err != nil {
		log.Sync()
		tail := make([]byte, 8192)
		var detail string
		if info, statErr := log.Stat(); statErr == nil {
			n, _ := log.ReadAt(tail, max(0, info.Size()-int64(len(tail))))
			lines := strings.Split(strings.TrimSpace(clean(string(tail[:n]), "")), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Preflight failed:") {
					detail = line
				}
			}
			if detail == "" && len(lines) > 0 {
				detail = lines[len(lines)-1]
			}
			if len(detail) > 300 {
				detail = detail[:300]
			}
		}
		operation := "Live environment check"
		if len(args) > 0 && args[0] == "--prepare-live" {
			operation = "Live environment preparation"
		}
		reason := operation + " failed: " + err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			reason = operation + " timed out"
		} else if ctx.Err() != nil {
			reason = operation + " was cancelled"
		}
		if detail != "" {
			reason += "\n" + detail
		}
		return fmt.Errorf("%s\nFull log: %s", reason, log.Name())
	}
	os.Remove(log.Name())
	return nil
}
func safeEnv() []string {
	env := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "HOME=/root", "TERM=dumb"}
	if executable, err := os.Executable(); err == nil {
		env = append(env, "VOID_INSTALLER_HELPER="+executable)
	}
	return env
}
func Start(parent context.Context, c Config, password string) (*Session, error) {
	if parent.Err() != nil {
		return nil, fmt.Errorf("Installation was cancelled before starting")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := validation.Password(password); err != nil {
		return nil, err
	}
	disks, err := DiscoverContext(parent)
	if err != nil {
		return nil, err
	}
	found := false
	for _, d := range disks {
		if d.Path == c.Disk && d.Identity == c.Identity && d.Safe {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("Disk is no longer safe or its identity changed; restart disk selection")
	}
	dir, err := extractEngine()
	if err != nil {
		return nil, err
	}
	session, err := startEngine(parent, c, password, filepath.Join(dir, "install.sh"), func() { os.RemoveAll(dir) })
	if err != nil {
		os.RemoveAll(dir)
	}
	return session, err
}
func startEngine(parent context.Context, c Config, password, script string, cleanup func()) (*Session, error) {
	if parent.Err() != nil {
		return nil, fmt.Errorf("Installation was cancelled before starting")
	}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("Cannot encode installation configuration")
	}
	secretR, secretW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		secretR.Close()
		secretW.Close()
		return nil, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		secretR.Close()
		secretW.Close()
		outR.Close()
		outW.Close()
		return nil, err
	}
	cmd := exec.Command("/bin/bash", script)
	cmd.Env = safeEnv()
	cmd.Stdin = bytes.NewReader(data)
	cmd.ExtraFiles = []*os.File{secretR}
	cmd.Stdout = outW
	cmd.Stderr = errW
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	log, err := newDiagnosticLog()
	if err != nil {
		for _, f := range []*os.File{secretR, secretW, outR, outW, errR, errW} {
			f.Close()
		}
		return nil, fmt.Errorf("Cannot create a private diagnostic log")
	}
	if err = cmd.Start(); err != nil {
		log.close()
		os.Remove(log.file.Name())
		for _, f := range []*os.File{secretR, secretW, outR, outW, errR, errW} {
			f.Close()
		}
		return nil, fmt.Errorf("Cannot start installation engine")
	}
	secretR.Close()
	outW.Close()
	errW.Close()
	ctx, cancel := context.WithCancel(parent)
	events := make(chan Event, 256)
	done := make(chan error, 1)
	s := &Session{Events: events, Done: done, LogPath: log.file.Name(), cancel: cancel, stopped: make(chan struct{})}
	go func() { defer secretW.Close(); io.WriteString(secretW, password+"\n") }()
	exited := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			select {
			case <-exited:
			case <-time.After(15 * time.Second):
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-exited:
		}
	}()
	go func() {
		defer close(s.stopped)
		defer cleanup()
		defer cancel()
		defer close(done)
		defer close(events)
		emit := func(e Event) {
			e.Message = clean(e.Message, password)
			log.write(e)
			select {
			case events <- e:
			case <-ctx.Done():
			}
		}
		var wg sync.WaitGroup
		var p protocol
		var protocolErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer outR.Close()
			scanner := bufio.NewScanner(outR)
			scanner.Buffer(make([]byte, 4096), 65536)
			for scanner.Scan() {
				e, er := ParseEvent(scanner.Bytes())
				if er == nil {
					er = p.accept(e)
				}
				if er != nil {
					protocolErr = er
					cancel()
					return
				}
				emit(e)
			}
			if scanner.Err() != nil {
				protocolErr = fmt.Errorf("Engine event stream exceeded its limit or failed")
				cancel()
			}
		}()
		go func() {
			defer wg.Done()
			defer errR.Close()
			scanner := bufio.NewScanner(errR)
			scanner.Buffer(make([]byte, 4096), 65536)
			for scanner.Scan() {
				emit(Event{Version: 1, Type: "log", Level: "info", Message: scanner.Text()})
			}
			if scanner.Err() != nil {
				emit(Event{Version: 1, Type: "log", Level: "warning", Message: "Diagnostic line exceeded its limit; remaining diagnostics discarded"})
				io.Copy(io.Discard, errR)
			}
		}()
		waitErr := cmd.Wait()
		close(exited)
		// Close inherited writers held by unexpected descendants before draining.
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		wg.Wait()
		var result error
		switch {
		case protocolErr != nil:
			result = protocolErr
		case ctx.Err() != nil:
			result = fmt.Errorf("Installation cancelled; target disk may be incomplete. Check remaining mounts before rebooting")
		case waitErr != nil:
			result = fmt.Errorf("Installation engine exited unsuccessfully")
		case !p.result || !p.success:
			result = fmt.Errorf("Installation engine exited without a verified success result")
		}
		if result != nil {
			log.write(Event{Version: 1, Type: "log", Level: "error", Message: result.Error()})
		}
		if log.close() != nil && result == nil {
			result = fmt.Errorf("Installation completed, but the diagnostic log could not be saved")
		}
		s.result = result
		done <- result
	}()
	return s, nil
}
func clean(text, password string) string {
	if password != "" {
		text = strings.ReplaceAll(text, password, "[REDACTED]")
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, text)
}

// Both subprocess streams share this writer, so os/exec serializes writes.
type preparationWriter struct {
	log     io.Writer
	report  func(string, string)
	pending string
}

func (w *preparationWriter) Write(p []byte) (int, error) {
	n, err := w.log.Write(p)
	if w.report == nil {
		return n, err
	}
	w.pending += string(p[:n])
	for {
		line, rest, ok := strings.Cut(w.pending, "\n")
		if !ok {
			break
		}
		w.pending = rest
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "VOID_PREP" {
			continue
		}
		switch fields[1] {
		case "environment", "tools", "xbps", "install":
		default:
			continue
		}
		switch fields[2] {
		case "running", "completed", "skipped":
		default:
			continue
		}
		w.report(fields[1], fields[2])
	}
	if len(w.pending) > 4096 {
		w.pending = ""
	}
	return n, err
}
