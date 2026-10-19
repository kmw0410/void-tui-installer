package installer

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestFilesystemDefaultsAndPackages(t *testing.T) {
	c := validConfig()
	if c.FilesystemName() != "btrfs" {
		t.Fatal("old configurations must retain Btrfs")
	}
	for _, fs := range []string{"btrfs", "ext4"} {
		c.Filesystem = fs
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
		packages := c.SoftwarePackages()
		if slices.Contains(packages, "snapper") != (fs == "btrfs") || slices.Contains(packages, "btrfs-progs") != (fs == "btrfs") || slices.Contains(packages, "e2fsprogs") != (fs == "ext4") {
			t.Fatal("incorrect filesystem dependencies", fs, packages)
		}
	}
	path := filepath.Join(t.TempDir(), "ext4.json")
	if err := SaveSettings(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSettings(path)
	if err != nil || loaded.FilesystemName() != "ext4" {
		t.Fatal("filesystem selection lost", err)
	}
	for _, fs := range []string{"xfs", "ext4\n", "../ext4"} {
		c.Filesystem = fs
		if c.Validate() == nil {
			t.Fatal("unsupported filesystem accepted", fs)
		}
	}
}

func TestSnapperSkipRequiresExt4Protocol(t *testing.T) {
	e := Event{Version: 1, Type: "stage", Name: "snapper", Status: "skipped"}
	for _, allowed := range []bool{false, true} {
		p := protocol{active: "snapper", next: 8, allowSnapshotSkip: allowed}
		err := p.accept(e)
		if (err == nil) != allowed {
			t.Fatal("incorrect snapshot skip policy", allowed, err)
		}
	}
	if _, err := ParseEvent([]byte(`{"version":1,"type":"stage","name":"verification","status":"skipped"}`)); err == nil {
		t.Fatal("mandatory verification was skippable")
	}
}
