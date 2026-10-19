package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ReviewChecks = []string{"environment", "network", "repository", "graphics", "disk", "capacity"}

type CheckReport struct {
	Status                   map[string]string
	RequiredBytes, DiskBytes uint64
}

func (r CheckReport) Passed() bool {
	for _, name := range ReviewChecks {
		if r.Status[name] != "passed" && !(name == "graphics" && r.Status[name] == "skipped") {
			return false
		}
	}
	const reserve = uint64(1073741824 + 2097152)
	return r.RequiredBytes > 0 && r.DiskBytes > reserve && r.DiskBytes-reserve >= r.RequiredBytes
}

// CheckInstallation validates the complete plan without a password or disk writes.
func CheckInstallation(ctx context.Context, c Config) (CheckReport, error) {
	c.Confirmation = "ERASE " + c.Disk
	if err := c.Validate(); err != nil {
		return CheckReport{}, err
	}
	dir, err := extractEngine()
	if err != nil {
		return CheckReport{}, err
	}
	defer os.RemoveAll(dir)
	data, err := json.Marshal(c)
	if err != nil {
		return CheckReport{}, err
	}
	var output packageOutput
	runErr := preflightRunInput(ctx, filepath.Join(dir, "install.sh"), nil, &output, data, "--check-install")
	report, err := parseCheckReport(output.Bytes())
	if runErr != nil {
		return report, runErr
	}
	if err != nil {
		return report, err
	}
	if !report.Passed() || (c.Graphics == "nvidia" && report.Status["graphics"] != "passed") {
		return report, fmt.Errorf("Installation checks did not complete")
	}
	return report, nil
}

func parseCheckReport(data []byte) (CheckReport, error) {
	r := CheckReport{Status: map[string]string{}}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	for {
		var e struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Required uint64 `json:"required_bytes"`
			Disk     uint64 `json:"disk_bytes"`
		}
		err := d.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return r, fmt.Errorf("Invalid installation check response")
		}
		known := false
		for _, name := range ReviewChecks {
			if e.Name == name {
				known = true
			}
		}
		if !known || (e.Status != "running" && e.Status != "passed" && !(e.Name == "graphics" && e.Status == "skipped")) {
			return r, fmt.Errorf("Invalid installation check status")
		}
		r.Status[e.Name] = e.Status
		if e.Name == "capacity" {
			r.RequiredBytes, r.DiskBytes = e.Required, e.Disk
		}
	}
	return r, nil
}
