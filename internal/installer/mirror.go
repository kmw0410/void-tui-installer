package installer

import (
	"fmt"
	"regexp"
	"strings"
)

const DefaultMirror = "https://repo-default.voidlinux.org"

type Mirror struct{ Name, URL string }

// These roots are published by Void's mirror directory, including Tier 2 mirrors.
var Mirrors = []Mirror{
	{"Default", DefaultMirror},
	{"Fastly CDN", "https://repo-fastly.voidlinux.org"},
	{"South Korea (Tier 2)", "https://mirror.heigou.pe.kr/voidlinux"},
	{"Japan (Tier 2)", "https://repo.jing.rocks/voidlinux"},
	{"Finland", "https://repo-fi.voidlinux.org"},
	{"Germany", "https://repo-de.voidlinux.org"},
}

func NormalizeMirror(value string) (string, error) {
	if value == "" {
		return DefaultMirror, nil
	}
	value = strings.TrimRight(value, "/")
	if len(value) > 512 || !regexp.MustCompile(`^https://[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?(:[0-9]{1,5})?(/[a-zA-Z0-9_+.-]+)*$`).MatchString(value) {
		return "", fmt.Errorf("Enter an HTTPS mirror root without credentials, query or fragment")
	}
	for _, part := range strings.Split(strings.TrimPrefix(value, "https://"), "/")[1:] {
		if part == "." || part == ".." || part == "current" || part == "nonfree" || part == "multilib" {
			return "", fmt.Errorf("Enter the mirror root only, such as https://mirror.example/voidlinux")
		}
	}
	return value, nil
}
