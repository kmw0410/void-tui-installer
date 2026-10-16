package installer

import "testing"

func TestMirrorRoots(t *testing.T) {
	for input, want := range map[string]string{"": DefaultMirror, "https://repo-fastly.voidlinux.org/": "https://repo-fastly.voidlinux.org", "https://mirror.example/voidlinux": "https://mirror.example/voidlinux", "https://mirror.example/void/": "https://mirror.example/void", "https://mirror.example:8443/pub/voidlinux": "https://mirror.example:8443/pub/voidlinux"} {
		got, err := NormalizeMirror(input)
		if err != nil || got != want {
			t.Fatalf("%q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{"http://mirror.example/void", "https://user:pass@mirror.example", "https://mirror.example/current", "https://mirror.example/voidlinux/current/nonfree", "https://mirror.example/../void", "https://mirror.example/%2e%2e", "https://mirror.example?x=1", "https://mirror.example/#x", "https://mirror.example\nrepository=bad", "https://mirror.example/void;reboot", "https://mirror.example/void\\x"} {
		if _, err := NormalizeMirror(input); err == nil {
			t.Fatal("unsafe or suffixed mirror accepted", input)
		}
	}
}
