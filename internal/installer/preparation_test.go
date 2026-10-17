package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparationMarkersSplitWritesAndValidation(t *testing.T) {
	var log bytes.Buffer
	var stages []string
	writer := preparationWriter{log: &log, report: func(stage, status string) { stages = append(stages, stage+":"+status) }}
	parts := []string{"ordinary package output\nVOID_PR", "EP environment running\nVOID_PREP unknown completed\n", "VOID_PREP tools invalid\nVOID_PREP tools completed\n"}
	for _, part := range parts {
		if _, err := writer.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(stages, ",") != "environment:running,tools:completed" {
		t.Fatalf("invalid progress: %v", stages)
	}
	if log.String() != strings.Join(parts, "") {
		t.Fatal("diagnostics must be preserved in full")
	}
	writer.Write([]byte(strings.Repeat("x", 5000)))
	if len(writer.pending) > 4096 {
		t.Fatal("partial-line buffering must remain bounded")
	}
}

func TestPreparationSubprocessReportsBeforeCompletion(t *testing.T) {
	script := filepath.Join(t.TempDir(), "prepare.sh")
	body := "printf 'VOID_PREP environment completed\\n' >&2\nprintf 'VOID_PREP xbps skipped\\n' >&2\nprintf 'VOID_PREP install skipped\\n' >&2\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	var stages []string
	err := preflightScriptProgress(context.Background(), script, func(stage, status string) { stages = append(stages, stage+":"+status) }, "--prepare-live")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(stages, ",") != "environment:completed,xbps:skipped,install:skipped" {
		t.Fatalf("missing subprocess markers: %v", stages)
	}
}
