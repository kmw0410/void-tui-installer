package installer

import (
	"slices"
	"testing"
	"time"
	_ "time/tzdata"
)

func TestTimezoneDataAndValidation(t *testing.T) {
	zones := Timezones()
	if len(zones) < 350 || !slices.IsSorted(zones) {
		t.Fatal("timezone list is incomplete or unordered")
	}
	for _, zone := range zones {
		if _, err := time.LoadLocation(zone); err != nil {
			t.Fatalf("invalid bundled timezone %q: %v", zone, err)
		}
	}
	c := validConfig()
	for _, zone := range []string{"", "UTC", "Asia/Seoul", "America/Argentina/Buenos_Aires"} {
		c.Timezone = zone
		if err := c.Validate(); err != nil {
			t.Fatal(zone, err)
		}
	}
	for _, zone := range []string{"../etc/passwd", "/etc/localtime", "Asia/Seoul\n", "Unknown/Zone", "Asia/Seoul;id"} {
		c.Timezone = zone
		if c.Validate() == nil {
			t.Fatal("invalid timezone accepted", zone)
		}
	}
}
