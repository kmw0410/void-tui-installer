package installer

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func engine(t *testing.T, body string) (*Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "engine.sh")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := startEngine(context.Background(), Config{}, "private secret123", path, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Cancel(); s.Wait(); os.Remove(s.LogPath) })
	return s, path
}
func TestUnexpectedExit(t *testing.T) {
	s, _ := engine(t, "exit 0\n")
	for range s.Events {
	}
	if <-s.Done == nil {
		t.Fatal("unexpected exit accepted")
	}
}

func TestPreflightFailureKeepsFullDiagnostics(t *testing.T) {
	script := filepath.Join(t.TempDir(), "check.sh")
	body := "printf 'The actual diagnostic before the package plan\\n' >&2\nfor i in {1..50}; do printf 'package-%s install x86_64 repository 123 456\\n' \"$i\"; done\nprintf 'Preflight failed: Package transaction resolution (exit status 23).\\n' >&2\nexit 23\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	err := preflightScript(context.Background(), script)
	if err == nil || !strings.Contains(err.Error(), "Package transaction resolution") || !strings.Contains(err.Error(), "exit status 23") {
		t.Fatalf("missing failing operation: %v", err)
	}
	_, path, ok := strings.Cut(err.Error(), "Full log: ")
	if !ok {
		t.Fatal("missing diagnostic log path")
	}
	t.Cleanup(func() { os.Remove(path) })
	data, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(data), "actual diagnostic before") || !strings.Contains(string(data), "package-50") {
		t.Fatal("full preflight diagnostics were lost")
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.Mode().Perm() != 0600 {
		t.Fatal("preflight diagnostic log is not private")
	}
}

func TestCancelledPreflightIsClearlyReported(t *testing.T) {
	script := filepath.Join(t.TempDir(), "check.sh")
	if err := os.WriteFile(script, []byte("sleep 60\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := preflightScript(ctx, script)
	if err == nil || !strings.Contains(err.Error(), "was cancelled") {
		t.Fatalf("ambiguous cancellation: %v", err)
	}
	if _, path, ok := strings.Cut(err.Error(), "Full log: "); ok {
		os.Remove(path)
	}
}

func TestPreflightModesPassMirrorAsOneArgument(t *testing.T) {
	script := filepath.Join(t.TempDir(), "mode.sh")
	if err := os.WriteFile(script, []byte("[[ $# == 1 && ( $1 == --check-local || $1 == --prepare-live ) ]] || [[ $# == 2 && $1 == --check && $2 == https://mirror.example/voidlinux ]]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--prepare-live"}, {"--check-local"}, {"--check", "https://mirror.example/voidlinux"}} {
		if err := preflightScript(context.Background(), script, args...); err != nil {
			t.Fatal(err)
		}
	}
}
func TestFailedExit(t *testing.T) {
	s, _ := engine(t, "exit 17\n")
	for range s.Events {
	}
	if <-s.Done == nil {
		t.Fatal("nonzero exit accepted")
	}
}
func TestMalformedEventStopsEngine(t *testing.T) {
	s, _ := engine(t, "printf 'broken\\n'\nsleep 60\n")
	select {
	case err := <-s.Done:
		if err == nil {
			t.Fatal("malformed event accepted")
		}
	case <-time.After(5 * time.Second):
		s.Cancel()
		t.Fatal("malformed engine was not stopped")
	}
}
func TestPasswordAndTerminalSanitization(t *testing.T) {
	s, _ := engine(t, "IFS= read -r secret <&3\nprintf '%s\\n' \"$secret\" >&2\nprintf '\\033[2Jcontrol\\n' >&2\nexit 1\n")
	for e := range s.Events {
		if strings.Contains(e.Message, "private secret") || strings.Contains(e.Message, "\x1b") {
			t.Fatal("unsafe diagnostic exposed")
		}
	}
	<-s.Done
	s.Wait()
	data, err := os.ReadFile(s.LogPath)
	if err != nil || strings.Contains(string(data), "private secret123") {
		t.Fatal("private log missing or exposed a password")
	}
	info, err := os.Stat(s.LogPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("diagnostic log is not private")
	}
}
func TestCancelReapsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	s, _ := engine(t, "trap 'exit 130' TERM\nsleep 60 &\nprintf '%s' \"$!\" > '"+pidFile+"'\nwait\n")
	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil {
			childPID, _ = strconv.Atoi(string(data))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		s.Cancel()
		t.Fatal("fixture child failed to start")
	}
	s.Cancel()
	select {
	case <-s.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel failed to reap engine")
	}
	if s.Wait() == nil {
		t.Fatal("cancel accepted as success")
	}
	if syscall.Kill(childPID, 0) == nil {
		status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(childPID), "status"))
		if err == nil && !strings.Contains(string(status), "State:\tZ") {
			t.Fatal("cancellation left a running child process")
		}
	}
}
func TestSuccessfulProtocol(t *testing.T) {
	var body strings.Builder
	for _, stage := range Stages {
		for _, status := range []string{"running", "completed"} {
			body.WriteString("printf '%s\\n' '{\"version\":1,\"type\":\"stage\",\"name\":\"" + stage + "\",\"status\":\"" + status + "\"}'\n")
		}
	}
	body.WriteString("printf '%s\\n' '{\"version\":1,\"type\":\"result\",\"status\":\"success\"}'\n")
	s, _ := engine(t, body.String())
	for range s.Events {
	}
	if e := <-s.Done; e != nil {
		t.Fatal(e)
	}
}
