package validation

import "testing"

func TestAccount(t *testing.T) {
	for _, v := range [][2]string{{"root", "void"}, {"a;id", "void"}, {"user", "-void"}, {"user", "void\nother"}, {"User", "void"}} {
		if Account(v[0], v[1]) == nil {
			t.Errorf("accepted invalid account %q", v)
		}
	}
	if Account("alice", "void-host") != nil {
		t.Fatal("valid account rejected")
	}
}
func TestPassword(t *testing.T) {
	for _, p := range []string{"short", "1234567", "has:colon1234", "has\nline12345", "unicode한글password"} {
		if Password(p) == nil {
			t.Fatal("invalid password accepted")
		}
	}
	if Password("12345678") != nil {
		t.Fatal("eight-character password rejected")
	}
	if Password("Correct horse$; 'battery") != nil {
		t.Fatal("shell metacharacters should be safe password data")
	}
}
func TestStatic(t *testing.T) {
	for _, v := range [][4]string{{"192.168.1.10/24", "192.168.1.1", "1.1.1.1", "eth0"}, {"10.0.0.10/8", "10.0.0.1", "8.8.8.8", "enp0s3"}} {
		if e := Network("static", v[3], v[0], v[1], v[2]); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range [][4]string{{"192.168.1.0/24", "192.168.1.1", "1.1.1.1", "eth0"}, {"192.168.1.255/24", "192.168.1.1", "1.1.1.1", "eth0"}, {"192.168.1.10/24", "10.0.0.1", "1.1.1.1", "eth0"}, {"192.168.1.10/32", "192.168.1.1", "1.1.1.1", "eth0"}, {"192.168.1.10/24", "192.168.1.1", "127.0.0.1", "eth0"}, {"192.168.1.10/24", "192.168.1.1", "1.1.1.1", "eth0;id"}} {
		if Network("static", v[3], v[0], v[1], v[2]) == nil {
			t.Errorf("unsafe network accepted %v", v)
		}
	}
	if Network("dhcp", "eth0", "", "", "") == nil {
		t.Fatal("DHCP retained static fields")
	}
}
