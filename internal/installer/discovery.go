package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Disk struct {
	Path, Model, Size, Type, Identity, Reason string
	Safe                                      bool
}
type blockDevice struct {
	Path        string        `json:"path"`
	Type        string        `json:"type"`
	Size        uint64        `json:"size"`
	Model       string        `json:"model"`
	Serial      string        `json:"serial"`
	WWN         string        `json:"wwn"`
	MajorMinor  string        `json:"maj:min"`
	RO          bool          `json:"ro"`
	RM          bool          `json:"rm"`
	Mountpoints []*string     `json:"mountpoints"`
	Children    []blockDevice `json:"children"`
}

func outputContext(parent context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s discovery failed", name)
	}
	return out, nil
}
func Discover() ([]Disk, error) {
	return DiscoverContext(context.Background())
}

func DiscoverContext(ctx context.Context) ([]Disk, error) {
	data, err := outputContext(ctx, "lsblk", "--json", "--tree", "--bytes", "--paths", "--output", "NAME,PATH,TYPE,SIZE,MODEL,MAJ:MIN,SERIAL,WWN,RO,RM,MOUNTPOINTS")
	if err != nil {
		return nil, err
	}
	devices, err := decodeInventory(data)
	if err != nil {
		return nil, err
	}
	var disks []Disk
	for _, d := range devices {
		if d.Type != "disk" {
			continue
		}
		disk := Disk{Path: d.Path, Model: clean(strings.TrimSpace(d.Model), ""), Type: d.Type, Size: fmt.Sprintf("%.1f GiB", float64(d.Size)/(1<<30))}
		disk.Reason = unsafeReason(d)
		if disk.Reason == "" {
			identity, e := outputContext(ctx, "lsblk", "-bdn", "-o", "MAJ:MIN,SIZE,LOG-SEC,SERIAL,WWN", "--", d.Path)
			if e != nil {
				disk.Reason = "Cannot read stable disk identity"
			} else {
				disk.Identity = strings.TrimSpace(string(identity))
			}
		}
		disk.Safe = disk.Reason == "" && disk.Identity != ""
		disks = append(disks, disk)
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Path < disks[j].Path })
	return disks, nil
}

func decodeInventory(data []byte) ([]blockDevice, error) {
	var tree struct {
		Devices []blockDevice `json:"blockdevices"`
	}
	if json.Unmarshal(data, &tree) != nil {
		return nil, fmt.Errorf("Invalid disk inventory")
	}
	for _, d := range tree.Devices {
		if d.Type == "part" {
			return nil, fmt.Errorf("Disk inventory lacks parent relationships; installation is blocked")
		}
	}
	return tree.Devices, nil
}
func unsafeReason(d blockDevice) string {
	if d.RO {
		return "Read-only device"
	}
	if d.RM {
		return "Removable media are protected"
	}
	if d.Size < 16<<30 {
		return "At least 16 GiB is required"
	}
	if strings.TrimSpace(d.Serial) == "" && strings.TrimSpace(d.WWN) == "" {
		return "Stable serial or WWN is required"
	}
	canonical, err := filepath.EvalSymlinks(d.Path)
	if err != nil || canonical != d.Path {
		return "Cannot identify canonical device"
	}
	return busyReason(d)
}
func busyReason(d blockDevice) string {
	if mounted(d) {
		return "Device or descendant is mounted or active swap"
	}
	dir := filepath.Join("/sys/dev/block", d.MajorMinor, "holders")
	holders, err := os.ReadDir(dir)
	if err != nil {
		return "Cannot inspect device holders"
	}
	if len(holders) > 0 {
		return "Device has active storage holders"
	}
	for _, child := range d.Children {
		if child.Type != "part" {
			return "Unsupported storage topology"
		}
		if r := busyReason(child); r != "" {
			return r
		}
	}
	return ""
}

func mounted(d blockDevice) bool {
	for _, m := range d.Mountpoints {
		if m != nil && *m != "" {
			return true
		}
	}
	for _, child := range d.Children {
		if mounted(child) {
			return true
		}
	}
	return false
}
func Interfaces() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, fmt.Errorf("Cannot enumerate network interfaces")
	}
	var names []string
	for _, e := range entries {
		if e.Name() == "lo" {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", e.Name(), "wireless")); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", e.Name(), "device")); err == nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
