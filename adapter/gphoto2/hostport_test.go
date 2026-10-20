package gphoto2

import "testing"

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		in      string
		addr    string
		port    int
		wantErr bool
	}{
		{"192.168.1.100", "192.168.1.100", 0, false},
		{"192.168.1.100:15740", "192.168.1.100", 15740, false},
		{"ptpip:192.168.1.100", "192.168.1.100", 0, false},
		{"ptpip:192.168.1.100:8080", "192.168.1.100", 8080, false},
		{"camera.local", "camera.local", 0, false},
		{"[fe80::1]", "fe80::1", 0, false},
		{"[fe80::1]:15740", "fe80::1", 15740, false},
		{"", "", 0, true},
		{"ptpip:", "", 0, true},
		{"[", "", 0, true},
	}
	for _, c := range cases {
		addr, port, err := splitHostPort(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("splitHostPort(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("splitHostPort(%q): %v", c.in, err)
		}
		if addr != c.addr || port != c.port {
			t.Fatalf("splitHostPort(%q) = (%q, %d), want (%q, %d)", c.in, addr, port, c.addr, c.port)
		}
	}
}
