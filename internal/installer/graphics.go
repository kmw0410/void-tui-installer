package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GPU struct{ Address, Device, SubVendor, SubDevice string }

func NVIDIAGPUs() ([]GPU, error) { return nvidiaGPUs("/sys/bus/pci/devices") }

func nvidiaGPUs(root string) ([]GPU, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("Cannot inspect PCI graphics hardware")
	}
	var devices []GPU
	for _, entry := range entries {
		read := func(name string) (string, error) {
			data, err := os.ReadFile(filepath.Join(root, entry.Name(), name))
			return strings.ToUpper(strings.TrimSpace(string(data))), err
		}
		class, err := read("class")
		if err != nil {
			return nil, fmt.Errorf("Cannot read PCI device class")
		}
		if !strings.HasPrefix(class, "0X03") {
			continue
		}
		vendor, err := read("vendor")
		if err != nil {
			return nil, fmt.Errorf("Cannot read graphics vendor")
		}
		if vendor != "0X10DE" {
			continue
		}
		g := GPU{Address: entry.Name()}
		for name, target := range map[string]*string{"device": &g.Device, "subsystem_vendor": &g.SubVendor, "subsystem_device": &g.SubDevice} {
			v, err := read(name)
			if err != nil || !regexp.MustCompile(`^0X[0-9A-F]{4}$`).MatchString(v) {
				return nil, fmt.Errorf("Cannot read NVIDIA hardware identity")
			}
			*target = v[2:]
		}
		devices = append(devices, g)
	}
	return devices, nil
}

func supportsGPUs(document string, devices []GPU) error {
	// NVIDIA appends unsupported legacy GPUs after the current-products table.
	start := strings.Index(document, `id="Current"`)
	if start < 0 {
		return fmt.Errorf("NVIDIA support document has no current-products section")
	}
	document = document[start:]
	start = strings.Index(document, "<tbody>")
	end := strings.Index(document, "</tbody>")
	if start < 0 || end <= start {
		return fmt.Errorf("Invalid NVIDIA current-products table")
	}
	rows := regexp.MustCompile(`<tr\s+id="devid([0-9A-Fa-f]{4})(?:_([0-9A-Fa-f]{4})_([0-9A-Fa-f]{4}))?"`).FindAllStringSubmatch(document[start:end], -1)
	if len(rows) == 0 {
		return fmt.Errorf("NVIDIA support document has no device identities")
	}
	for _, g := range devices {
		found := false
		for _, row := range rows {
			if strings.EqualFold(row[1], g.Device) && (row[2] == "" || strings.EqualFold(row[2], g.SubVendor) && strings.EqualFold(row[3], g.SubDevice)) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("NVIDIA %s at %s is unsupported by the repository driver; use the default driver", g.Device, g.Address)
		}
	}
	return nil
}

func CheckNVIDIA(ctx context.Context, version string) error {
	if !regexp.MustCompile(`^[0-9]{3}\.[0-9]{2,3}(\.[0-9]{2})?$`).MatchString(version) {
		return fmt.Errorf("Invalid NVIDIA driver version")
	}
	devices, err := NVIDIAGPUs()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("No NVIDIA display controller is present")
	}
	url := "https://download.nvidia.com/XFree86/Linux-x86_64/" + version + "/README/supportedchips.html"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return fmt.Errorf("Unexpected NVIDIA documentation redirect")
	}}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Cannot verify NVIDIA GPU support: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("NVIDIA support lookup failed: HTTP %d", response.StatusCode)
	}
	document, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if err != nil || len(document) > 2*1024*1024 {
		return fmt.Errorf("Invalid NVIDIA support document")
	}
	return supportsGPUs(string(document), devices)
}
