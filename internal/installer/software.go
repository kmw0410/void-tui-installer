package installer

import (
	"encoding/json"
	"fmt"
	"slices"
	"void-installer/scripts"
)

type softwareCatalog struct {
	Base        []string            `json:"base"`
	Filesystems map[string][]string `json:"filesystems"`
	Common      []string            `json:"common"`
	Utilities   []string            `json:"utilities"`
	Graphical   []string            `json:"graphical"`
	Fcitx       []string            `json:"fcitx"`
	Desktops    map[string][]string `json:"desktops"`
}

var catalog = func() softwareCatalog {
	data, err := scripts.FS.ReadFile("catalog.json")
	if err != nil {
		panic(err)
	}
	var c softwareCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		panic(err)
	}
	return c
}()

func UtilityPackages() []string { return slices.Clone(catalog.Utilities) }

func (c Config) ValidateSoftware() error {
	// Empty values preserve compatibility with the original CLI-only profile.
	if c.Desktop != "" {
		if _, ok := catalog.Desktops[c.Desktop]; !ok {
			return fmt.Errorf("Invalid desktop selection")
		}
	}
	if c.Graphics != "" && c.Graphics != "default" && c.Graphics != "nvidia" {
		return fmt.Errorf("Invalid graphics selection")
	}
	seen := map[string]bool{}
	if len(c.Utilities) > MaxExtraPackages {
		return fmt.Errorf("Select at most %d additional packages", MaxExtraPackages)
	}
	for _, p := range c.Utilities {
		if !packageNamePattern.MatchString(p) || seen[p] {
			return fmt.Errorf("Invalid or duplicate package name")
		}
		seen[p] = true
	}
	return nil
}

func (c Config) SoftwarePackages() []string {
	packages := slices.Clone(catalog.Base)
	packages = append(packages, catalog.Filesystems[c.FilesystemName()]...)
	packages = append(packages, catalog.Common...)
	if c.Desktop != "" && c.Desktop != "none" {
		packages = append(packages, catalog.Graphical...)
		packages = append(packages, catalog.Desktops[c.Desktop]...)
	}
	if c.Graphics == "nvidia" {
		packages = append(packages, "nvidia", "linux-headers", "void-repo-nonfree")
	}
	packages = append(packages, c.Utilities...)
	for _, p := range c.Utilities {
		if len(p) >= 6 && p[:6] == "fcitx5" {
			packages = append(packages, catalog.Fcitx...)
			break
		}
	}
	unique := make([]string, 0, len(packages))
	seen := map[string]bool{}
	for _, p := range packages {
		if !seen[p] {
			unique = append(unique, p)
			seen[p] = true
		}
	}
	return unique
}
