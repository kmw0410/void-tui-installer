package installer

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRepositoryPackageParsing(t *testing.T) {
	data := "[-] zathura-pdf-mupdf-0.3.9_1    PDF plugin\n[-] 0ad-0.27.1_6 Game\n[*] ripgrep-14.1.1_1 Fast search\n[-] ripgrep-14.1.1_1 Duplicate\n"
	packages, err := parseRepositoryPackages([]byte(data))
	if err != nil || len(packages) != 3 {
		t.Fatalf("package parsing failed: %v %v", packages, err)
	}
	if packages[0].Name != "0ad" || packages[2].Name != "zathura-pdf-mupdf" || packages[2].Version != "0.3.9_1" || packages[2].Description != "PDF plugin" {
		t.Fatal("package names, versions or descriptions changed", packages)
	}
	if err := ValidateRepositorySelection([]string{"ripgrep"}, packages); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "ripgrep-14.1.1_1", "ripgrep>=14", "--force"} {
		if ValidateRepositorySelection([]string{name}, packages) == nil {
			t.Fatal("accepted package outside the exact repository list", name)
		}
	}
	for _, input := range []string{"", "error", "[-] bad\n", "[-] --force-1_1 bad\n", "[-] good-1\x00_1 bad\n", "[-] valid-1_1 valid\nmalformed\n"} {
		if _, err := parseRepositoryPackages([]byte(input)); err == nil {
			t.Fatal("invalid repository output accepted", input)
		}
	}
}

func TestRepositoryPackageLimits(t *testing.T) {
	var w packageOutput
	if _, err := w.Write(make([]byte, 16*1024*1024+1)); err == nil {
		t.Fatal("unbounded package output accepted")
	}
	c := validConfig()
	c.Utilities = []string{"zathura-pdf-mupdf"}
	if err := c.Validate(); err != nil {
		t.Fatal("repository packages must not require the old fixed list", err)
	}
	for _, name := range []string{strings.Repeat("a", 129), "pkg>=1", "pkg*", "-option", "pkg name"} {
		c.Utilities = []string{name}
		if c.Validate() == nil {
			t.Fatal("invalid package expression accepted", name)
		}
	}
	c.Utilities = nil
	for i := 0; i <= MaxExtraPackages; i++ {
		c.Utilities = append(c.Utilities, fmt.Sprintf("package-%d", i))
	}
	if c.Validate() == nil {
		t.Fatal("too many selected packages accepted")
	}
}

func TestRepositoryPackageSnapshot(t *testing.T) {
	path := os.Getenv("VOID_TEST_PACKAGE_LIST")
	if path == "" {
		t.Skip("optional read-only XBPS repository snapshot not supplied")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	packages, err := parseRepositoryPackages(data)
	if err != nil || len(packages) < 1000 {
		t.Fatalf("real repository list failed validation: %d packages, %v", len(packages), err)
	}
	t.Logf("Parsed %d repository packages", len(packages))
}
