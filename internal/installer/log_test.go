package installer

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDiagnosticLogRotationPreservesRecentFailures(t *testing.T) {
	log, err := newDiagnosticLog()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(log.file.Name()) })
	for i := 0; i < 280; i++ {
		log.write(Event{Version: 1, Type: "log", Level: "info", Message: strings.Repeat("x", 32768)})
	}
	log.write(Event{Version: 1, Type: "log", Level: "error", Message: "Cleanup failed: inspect remaining mounts"})
	if err := log.close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(log.file.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 8<<20 {
		t.Fatal("diagnostic log grew beyond its bound")
	}
	scanner := bufio.NewScanner(f)
	var last Event
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &last); err != nil {
			t.Fatal("rotation damaged the JSON Lines log")
		}
	}
	if scanner.Err() != nil || !strings.Contains(last.Message, "Cleanup failed") {
		t.Fatal("rotation lost the recent failure")
	}
}
