package installer

import (
	"encoding/json"
	"fmt"
	"slices"
	"void-installer/scripts"
)

// Timezone names come from the public-domain IANA zone.tab, plus UTC.
var timezones = func() []string {
	data, err := scripts.FS.ReadFile("timezones.json")
	if err != nil {
		panic(err)
	}
	var zones []string
	if err := json.Unmarshal(data, &zones); err != nil {
		panic(err)
	}
	return zones
}()

func Timezones() []string { return slices.Clone(timezones) }

func (c Config) TimezoneName() string {
	if c.Timezone == "" {
		return "UTC"
	}
	return c.Timezone
}

func validateTimezone(zone string) error {
	if _, ok := slices.BinarySearch(timezones, zone); !ok {
		return fmt.Errorf("Select a timezone from the list")
	}
	return nil
}
