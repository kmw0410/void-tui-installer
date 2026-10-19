package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestSettingsRoundTripExcludesConsentAndSecrets(t *testing.T) {
	c := validConfig()
	c.Utilities = []string{"vim", "zathura"}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := SaveSettings(path, c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "confirmation") || strings.Contains(string(data), "ERASE") || strings.Contains(strings.ToLower(string(data)), "password") {
		t.Fatal("unsafe field persisted")
	}
	got, err := LoadSettings(path)
	c.Confirmation = ""
	if err != nil || !reflect.DeepEqual(c, got) {
		t.Fatalf("settings changed: %#v %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("settings are not private")
	}
	c.Hostname = "newhost"
	if err := SaveSettings(path, c); err != nil {
		t.Fatal("valid settings replacement failed", err)
	}
	got, err = LoadSettings(path)
	if err != nil || got.Hostname != "newhost" {
		t.Fatal("replacement not loaded")
	}
}

func TestSettingsRejectUnsafeFilesAndData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unrelated")
	if err := os.WriteFile(path, []byte("unrelated data"), 0644); err != nil {
		t.Fatal(err)
	}
	if SaveSettings(path, validConfig()) == nil {
		t.Fatal("unrelated file overwritten")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(link); err == nil {
		t.Fatal("symlink loaded")
	}
	if SaveSettings(link, validConfig()) == nil {
		t.Fatal("symlink replaced")
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(fifo); err == nil {
		t.Fatal("non-regular settings loaded")
	}
	data, _ := settingsBytes(validConfig())
	for _, field := range []string{"confirmation", "password", "root_password"} {
		var saved settingsFile
		_ = json.Unmarshal(data, &saved)
		saved.Configuration[field] = json.RawMessage(`"secret"`)
		bad, _ := json.Marshal(saved)
		if _, err := decodeSettings(bad); err == nil {
			t.Fatal("unsupported field loaded", field)
		}
	}
	for _, bad := range [][]byte{append(data, []byte(" {}")...), []byte(`{"settings_version":2,"configuration":{}}`), []byte(`{"settings_version":1,"configuration":null}`)} {
		if _, err := decodeSettings(bad); err == nil {
			t.Fatal("invalid settings loaded")
		}
	}
	if err := os.WriteFile(path, make([]byte, settingsLimit+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(path); err == nil {
		t.Fatal("oversized settings loaded")
	}
}
