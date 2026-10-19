package installer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const MaxExtraPackages = 256

var packageNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9+_.-]{0,127}$`)

type Package struct {
	Name, Version, Description string
}

// RepositoryPackages uses XBPS signature verification in a private metadata root.
func RepositoryPackages(ctx context.Context, mirror string) ([]Package, error) {
	root, err := NormalizeMirror(mirror)
	if err != nil {
		return nil, err
	}
	dir, err := extractEngine()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	var output packageOutput
	if err := preflightRun(ctx, filepath.Join(dir, "install.sh"), nil, &output, "--list-packages", root); err != nil {
		return nil, err
	}
	return parseRepositoryPackages(output.Bytes())
}

type packageOutput struct{ bytes.Buffer }

func (w *packageOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 16*1024*1024 {
		return 0, fmt.Errorf("Repository package list exceeds its size limit")
	}
	return w.Buffer.Write(p)
}

func parseRepositoryPackages(data []byte) ([]Package, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 65536)
	packages := make([]Package, 0)
	seen := map[string]bool{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "[-] ") && !strings.HasPrefix(line, "[*] ") {
			return nil, fmt.Errorf("Invalid repository package list")
		}
		pkgver, description, ok := strings.Cut(strings.TrimSpace(line[4:]), " ")
		separator := strings.LastIndexByte(pkgver, '-')
		if !ok || separator <= 0 || separator == len(pkgver)-1 {
			return nil, fmt.Errorf("Invalid repository package record")
		}
		name, version := pkgver[:separator], pkgver[separator+1:]
		if !packageNamePattern.MatchString(name) || strings.ContainsFunc(version, func(r rune) bool { return r <= 32 || r >= 127 }) {
			return nil, fmt.Errorf("Invalid repository package name or version")
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		packages = append(packages, Package{Name: name, Version: version, Description: strings.TrimSpace(clean(description, ""))})
		if len(packages) > 100000 {
			return nil, fmt.Errorf("Repository contains too many packages")
		}
	}
	if scanner.Err() != nil || len(packages) == 0 {
		return nil, fmt.Errorf("Repository package list is empty or incomplete")
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return packages, nil
}

func ValidateRepositorySelection(selected []string, packages []Package) error {
	available := make(map[string]bool, len(packages))
	for _, p := range packages {
		available[p.Name] = true
	}
	for _, name := range selected {
		if !available[name] {
			return fmt.Errorf("Package is unavailable in the selected main repository: %s", name)
		}
	}
	return nil
}
