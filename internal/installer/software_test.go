package installer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSoftwareAllowlist(t *testing.T) {
	c := validConfig()
	for _, p := range UtilityPackages() {
		c.Utilities = []string{p}
		if err := c.Validate(); err != nil {
			t.Fatal(p, err)
		}
	}
	for _, p := range []string{"--force", "firefox;reboot", "unknown-package", "firefox\nkitty"} {
		c.Utilities = []string{p}
		if c.Validate() == nil {
			t.Fatal("unsafe utility accepted", p)
		}
	}
	c.Utilities = []string{"firefox", "firefox"}
	if c.Validate() == nil {
		t.Fatal("duplicate utility accepted")
	}
	c.Utilities = nil
	for _, desktop := range []string{"none", "kde", "niri"} {
		c.Desktop = desktop
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
		packages := c.SoftwarePackages()
		for _, p := range []string{"bluez", "tlp"} {
			if !slices.Contains(packages, p) {
				t.Fatal("common package missing", desktop, p)
			}
		}
		if slices.Contains(packages, "firefox") || slices.Contains(packages, "kitty") || slices.Contains(packages, "kde-plasma") {
			t.Fatal("unselected apps or full KDE bundle added")
		}
		if desktop != "none" && !slices.Contains(packages, "sddm") {
			t.Fatal("missing display manager")
		}
		if slices.Contains(packages, "kscreen") != (desktop == "kde") {
			t.Fatal("display settings must be included in the KDE profile")
		}
		for _, p := range []string{"pipewire", "wireplumber", "libjack-pipewire", "alsa-pipewire", "xdg-user-dirs", "noto-fonts-cjk", "NetworkManager"} {
			if slices.Contains(packages, p) != (desktop != "none") {
				t.Fatal("audio profile mismatch", desktop, p)
			}
		}
		if desktop == "kde" {
			for _, p := range []string{"konsole", "sddm-kcm", "plasma-pa", "kwallet-pam"} {
				if !slices.Contains(packages, p) {
					t.Fatal("KDE integration missing", p)
				}
			}
		}
		if desktop == "niri" {
			for _, p := range []string{"xdg-desktop-portal-gnome", "gnome-keyring", "seahorse", "xwayland-satellite", "network-manager-applet", "polkit-gnome", "pavucontrol"} {
				if !slices.Contains(packages, p) {
					t.Fatal("niri integration missing", p)
				}
			}
		}
	}
	c.Utilities = []string{"fcitx5", "fcitx5-hangul"}
	count := 0
	for _, p := range c.SoftwarePackages() {
		if p == "fcitx5" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("implicit input-method dependency was duplicated")
	}
	for _, selection := range []string{"fcitx5", "fcitx5-hangul"} {
		c.Utilities = []string{selection}
		for _, p := range catalog.Fcitx {
			if !slices.Contains(c.SoftwarePackages(), p) {
				t.Fatal("input bundle missing", selection, p)
			}
		}
	}
	if len(UtilityPackages()) != 20 {
		t.Fatal("toolkit module packages must not be separate utility choices")
	}
}

func TestNVIDIACurrentTableAndSubsystem(t *testing.T) {
	doc := `<a id="Current"></a><tbody><tr id="devid2684"><td>GPU</td></tr><tr id="devid1234_1028_5678"></tr></tbody><tbody><tr id="devid0BAD"></tr></tbody>`
	if err := supportsGPUs(doc, []GPU{{Device: "2684"}, {Device: "1234", SubVendor: "1028", SubDevice: "5678"}}); err != nil {
		t.Fatal(err)
	}
	for _, g := range []GPU{{Device: "0BAD"}, {Device: "1234", SubVendor: "0000", SubDevice: "5678"}} {
		if supportsGPUs(doc, []GPU{g}) == nil {
			t.Fatal("unsupported GPU accepted")
		}
	}
	if supportsGPUs("invalid", []GPU{{Device: "2684"}}) == nil {
		t.Fatal("malformed document accepted")
	}
}

func TestNVIDIADiscoveryIgnoresAudio(t *testing.T) {
	root := t.TempDir()
	for name, class := range map[string]string{"0000:01:00.0": "0x030000", "0000:01:00.1": "0x040300"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for key, value := range map[string]string{"class": class, "vendor": "0x10de", "device": "0x2684", "subsystem_vendor": "0x1028", "subsystem_device": "0x5678"} {
			if err := os.WriteFile(filepath.Join(dir, key), []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	devices, err := nvidiaGPUs(root)
	if err != nil || len(devices) != 1 || devices[0].Device != "2684" {
		t.Fatal(devices, err)
	}
}
