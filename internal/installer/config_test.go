package installer

import (
	"encoding/json"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{Version: 1, Disk: "/dev/vda", Identity: "253:0 25769803776 512 VOIDTEST001", Username: "alice", Hostname: "void", Network: "dhcp", Confirmation: "ERASE /dev/vda"}
}

func TestConfigConsentAndIdentity(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Confirmation = "yes" },
		func(c *Config) { c.Confirmation = "ERASE /dev/vdb" },
		func(c *Config) { c.Identity = "" },
		func(c *Config) { c.Identity = "changed\nidentity" },
		func(c *Config) { c.Identity = "changed\tidentity" },
		func(c *Config) { c.Identity = strings.Repeat("x", 1025) },
		func(c *Config) { c.Disk = "/dev/disk/by-id/example" },
		func(c *Config) { c.Username = "alice\n" },
		func(c *Config) { c.Confirmation += "\n" },
		func(c *Config) { c.Desktop = "none\x00" },
		func(c *Config) { c.Mirror = "https://repo-default.voidlinux.org\n" },
		func(c *Config) { c.Disk = "/dev/../etc/passwd" },
		func(c *Config) { c.Version = 2 },
	} {
		c := validConfig()
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal("unsafe installation configuration accepted")
		}
	}
}

func TestConfigCannotSerializeCredentials(t *testing.T) {
	data, err := json.Marshal(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "password") {
		t.Fatal("credential field in public configuration protocol")
	}
}

func TestUnsafeDiskInventory(t *testing.T) {
	for _, d := range []blockDevice{
		{RO: true},
		{RM: true},
		{Size: 1 << 30},
		{Size: 32 << 30},
	} {
		if unsafeReason(d) == "" {
			t.Fatal("unidentifiable or ineligible disk accepted")
		}
	}
	mount := "/run/initramfs/live"
	if busyReason(blockDevice{Mountpoints: []*string{&mount}}) == "" {
		t.Fatal("live-medium mount accepted")
	}
}

func TestMountedDescendantsAndFlattenedInventory(t *testing.T) {
	flat := []byte(`{"blockdevices":[{"path":"/dev/vda","type":"disk"},{"path":"/dev/vda2","type":"part","mountpoints":["/live"]}]}`)
	if _, err := decodeInventory(flat); err == nil {
		t.Fatal("flattened inventory must fail closed instead of ignoring partitions")
	}
	tree := []byte(`{"blockdevices":[{"path":"/dev/vda","type":"disk","mountpoints":[null],"children":[{"path":"/dev/vda2","type":"part","mountpoints":["/live"]}]}]}`)
	devices, err := decodeInventory(tree)
	if err != nil || len(devices) != 1 {
		t.Fatalf("tree inventory failed: %v", err)
	}
	if !mounted(devices[0]) || busyReason(devices[0]) != "Device or descendant is mounted or active swap" {
		t.Fatal("mounted child partition must block the entire disk")
	}
}
