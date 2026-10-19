package installer

import (
	"fmt"
	"regexp"
	"strings"
	"void-installer/internal/validation"
)

type Config struct {
	Version      int      `json:"version"`
	Disk         string   `json:"disk"`
	Identity     string   `json:"identity"`
	Username     string   `json:"username"`
	Hostname     string   `json:"hostname"`
	Network      string   `json:"network"`
	Interface    string   `json:"interface"`
	Address      string   `json:"address"`
	Gateway      string   `json:"gateway"`
	DNS          string   `json:"dns"`
	Confirmation string   `json:"confirmation"`
	Desktop      string   `json:"desktop"`
	Graphics     string   `json:"graphics"`
	Utilities    []string `json:"utilities"`
	Mirror       string   `json:"mirror"`
	Timezone     string   `json:"timezone"`
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("Unsupported configuration version")
	}
	if !regexp.MustCompile(`^/dev/[a-zA-Z0-9_-]+$`).MatchString(c.Disk) {
		return fmt.Errorf("Invalid disk path")
	}
	for _, value := range append([]string{c.Disk, c.Identity, c.Username, c.Hostname, c.Network, c.Interface, c.Address, c.Gateway, c.DNS, c.Confirmation, c.Desktop, c.Graphics, c.Mirror, c.Timezone}, c.Utilities...) {
		if strings.ContainsFunc(value, func(r rune) bool { return r < 32 || r == 127 }) {
			return fmt.Errorf("Configuration strings must not contain control characters")
		}
	}
	if c.Identity == "" || len(c.Identity) > 1024 {
		return fmt.Errorf("Disk identity is unavailable")
	}
	if err := validation.Account(c.Username, c.Hostname); err != nil {
		return err
	}
	if err := validation.Network(c.Network, c.Interface, c.Address, c.Gateway, c.DNS); err != nil {
		return err
	}
	if err := c.ValidateSoftware(); err != nil {
		return err
	}
	if _, err := NormalizeMirror(c.Mirror); err != nil {
		return err
	}
	if err := validateTimezone(c.TimezoneName()); err != nil {
		return err
	}
	if c.Confirmation != "ERASE "+c.Disk {
		return fmt.Errorf("Type ERASE followed by the exact disk path to confirm data loss")
	}
	return nil
}
