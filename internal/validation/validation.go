package validation

import (
	"fmt"
	"net/netip"
	"regexp"
)

var username = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,30}$`)
var hostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
var iface = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,14}$`)

func Account(user, host string) error {
	if !username.MatchString(user) || user == "root" {
		return fmt.Errorf("Username must be a non-root lowercase account name (1–31 characters)")
	}
	if !hostname.MatchString(host) {
		return fmt.Errorf("Hostname must be a single DNS label (1–63 characters)")
	}
	return nil
}
func Password(value string) error {
	if len(value) < 8 || len(value) > 128 {
		return fmt.Errorf("Password must contain 8–128 printable ASCII characters")
	}
	for _, c := range value {
		if c < 32 || c > 126 || c == ':' {
			return fmt.Errorf("Password cannot contain colons or control characters")
		}
	}
	return nil
}
func Network(mode, device, address, gateway, dns string) error {
	if mode == "dhcp" {
		if device != "" || address != "" || gateway != "" || dns != "" {
			return fmt.Errorf("DHCP configuration must not contain static fields")
		}
		return nil
	}
	if mode != "static" {
		return fmt.Errorf("Select DHCP or static networking")
	}
	if !iface.MatchString(device) || device == "lo" {
		return fmt.Errorf("Select a valid network interface")
	}
	prefix, e := netip.ParsePrefix(address)
	if e != nil || !prefix.Addr().Is4() || prefix.Bits() < 1 || prefix.Bits() > 30 || !unicast(prefix.Addr()) {
		return fmt.Errorf("Enter a usable IPv4 address with a CIDR prefix from 1 to 30")
	}
	ip := prefix.Addr().As4()
	mask := uint32(0xffffffff) << uint(32-prefix.Bits())
	n := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
	if n & ^mask == 0 || n & ^mask == ^mask {
		return fmt.Errorf("IP address cannot be a network or broadcast address")
	}
	gw, e := netip.ParseAddr(gateway)
	if e != nil || !gw.Is4() || !unicast(gw) || !prefix.Contains(gw) || gw == prefix.Addr() {
		return fmt.Errorf("Gateway must be a different usable IPv4 address in the same subnet")
	}
	g := gw.As4()
	gn := uint32(g[0])<<24 | uint32(g[1])<<16 | uint32(g[2])<<8 | uint32(g[3])
	if gn & ^mask == 0 || gn & ^mask == ^mask {
		return fmt.Errorf("Gateway cannot be a network or broadcast address")
	}
	d, e := netip.ParseAddr(dns)
	if e != nil || !d.Is4() || !unicast(d) {
		return fmt.Errorf("Enter a unicast IPv4 DNS server")
	}
	return nil
}
func unicast(a netip.Addr) bool {
	return a.IsGlobalUnicast() && !a.IsLoopback() && !a.IsLinkLocalUnicast()
}
