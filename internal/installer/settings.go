package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const settingsLimit = 65536

// Only these public fields may cross the settings-file boundary.
var settingsFields = []string{"version", "disk", "identity", "username", "hostname", "network", "interface", "address", "gateway", "dns", "desktop", "graphics", "utilities", "mirror", "timezone"}

type settingsFile struct {
	Version       int                        `json:"settings_version"`
	Configuration map[string]json.RawMessage `json:"configuration"`
}

func validateSettings(c Config) error {
	c.Confirmation = "ERASE " + c.Disk
	return c.Validate()
}

func settingsBytes(c Config) ([]byte, error) {
	if err := validateSettings(c); err != nil {
		return nil, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(data, &source); err != nil {
		return nil, err
	}
	public := make(map[string]json.RawMessage, len(settingsFields))
	for _, name := range settingsFields {
		public[name] = source[name]
	}
	return json.MarshalIndent(settingsFile{Version: 1, Configuration: public}, "", "  ")
}

func decodeSettings(data []byte) (Config, error) {
	var saved settingsFile
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&saved); err != nil {
		return Config{}, fmt.Errorf("Invalid settings file")
	}
	var extra any
	if d.Decode(&extra) != io.EOF || saved.Version != 1 || saved.Configuration == nil {
		return Config{}, fmt.Errorf("Unsupported or trailing settings data")
	}
	allowed := map[string]bool{}
	for _, name := range settingsFields {
		allowed[name] = true
	}
	for name := range saved.Configuration {
		if !allowed[name] {
			return Config{}, fmt.Errorf("Settings contain an unsupported field")
		}
	}
	data, err := json.Marshal(saved.Configuration)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("Invalid settings values")
	}
	if err := validateSettings(c); err != nil {
		return Config{}, err
	}
	c.Confirmation = ""
	return c, nil
}

func LoadSettings(path string) (Config, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return Config{}, fmt.Errorf("Cannot open settings: %w", err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > settingsLimit {
		return Config{}, fmt.Errorf("Settings must be a regular file of at most 64 KiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, settingsLimit+1))
	if err != nil || len(data) > settingsLimit {
		return Config{}, fmt.Errorf("Cannot read settings within its size limit")
	}
	return decodeSettings(data)
}

// SaveSettings replaces valid settings atomically; unrelated files are preserved.
func SaveSettings(path string, c Config) error {
	data, err := settingsBytes(c)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Settings destination must be a regular file")
		}
		if _, err := LoadSettings(path); err != nil {
			return fmt.Errorf("Refusing to overwrite a file that is not valid installer settings")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Cannot inspect settings destination: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".void-installer-settings-*")
	if err != nil {
		return fmt.Errorf("Cannot create private settings file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
