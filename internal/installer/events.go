package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

var Stages = []string{"preflight", "partitioning", "filesystems", "bootstrap", "system", "network", "software", "grub", "snapper", "verification", "cleanup"}

type Event struct {
	Version int    `json:"version"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
}

func ParseEvent(line []byte) (Event, error) {
	var e Event
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return e, fmt.Errorf("Invalid engine event")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return e, fmt.Errorf("Trailing engine event data")
	}
	if e.Version != 1 {
		return e, fmt.Errorf("Unsupported engine protocol")
	}
	switch e.Type {
	case "stage":
		valid := false
		for _, s := range Stages {
			if e.Name == s {
				valid = true
			}
		}
		if !valid || (e.Status != "running" && e.Status != "completed" && e.Status != "failed") || e.Level != "" {
			return e, fmt.Errorf("Invalid stage event")
		}
	case "log":
		if (e.Level != "info" && e.Level != "warning" && e.Level != "error") || e.Name != "" || e.Status != "" {
			return e, fmt.Errorf("Invalid log event")
		}
	case "result":
		if (e.Status != "success" && e.Status != "failed" && e.Status != "cancelled") || e.Name != "" || e.Level != "" {
			return e, fmt.Errorf("Invalid result event")
		}
	default:
		return e, fmt.Errorf("Unknown engine event type")
	}
	return e, nil
}

type protocol struct {
	next                    int
	active                  string
	failed, result, success bool
}

func (p *protocol) accept(e Event) error {
	if p.result {
		return fmt.Errorf("Engine emitted an event after its result")
	}
	switch e.Type {
	case "stage":
		if e.Status == "running" {
			if p.active != "" || p.next >= len(Stages) || (e.Name != Stages[p.next] && !(p.failed && e.Name == "cleanup")) {
				return fmt.Errorf("Invalid engine stage order")
			}
			p.active = e.Name
		} else {
			if e.Name != p.active {
				return fmt.Errorf("Engine completed an inactive stage")
			}
			p.active = ""
			if e.Status == "failed" {
				p.failed = true
			}
			if e.Name == "cleanup" {
				p.next = len(Stages)
			} else {
				p.next++
			}
		}
	case "result":
		p.result = true
		p.success = e.Status == "success"
		if p.success && (p.failed || p.active != "" || p.next != len(Stages)) {
			return fmt.Errorf("Engine reported success before verification and cleanup")
		}
	}
	return nil
}
