package installer

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckReportCompletionAndPartialFailure(t *testing.T) {
	var b strings.Builder
	for _, name := range ReviewChecks {
		if name == "capacity" {
			b.WriteString("{\"name\":\"capacity\",\"status\":\"passed\",\"required_bytes\":4000000000,\"disk_bytes\":32000000000}\n")
		} else {
			fmt.Fprintf(&b, "{\"name\":%q,\"status\":\"passed\"}\n", name)
		}
	}
	r, err := parseCheckReport([]byte(b.String()))
	if err != nil || !r.Passed() {
		t.Fatal("complete checks failed", r, err)
	}
	r, err = parseCheckReport([]byte("{\"name\":\"environment\",\"status\":\"passed\"}\n{\"name\":\"repository\",\"status\":\"running\"}\n"))
	if err != nil || r.Passed() || r.Status["repository"] != "running" {
		t.Fatal("partial failure lost", r, err)
	}
	for _, bad := range []string{`{"name":"unknown","status":"passed"}`, `{"name":"disk","status":"skipped"}`, `{"name":"disk","status":"passed","extra":true}`, `malformed`} {
		if _, err := parseCheckReport([]byte(bad)); err == nil {
			t.Fatal("invalid report accepted", bad)
		}
	}
}
