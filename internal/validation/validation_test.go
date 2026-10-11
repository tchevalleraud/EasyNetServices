package validation

import "testing"

func TestIsHost(t *testing.T) {
	valid := []string{"pool.ntp.org", "time.cloudflare.com.", "ntp1", "192.168.1.1", "2001:db8::1"}
	invalid := []string{"", "bad host", "evil\nallow all", "-start.example", "a..b", "host_name.example"}
	for _, s := range valid {
		if !IsHost(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if IsHost(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
