package accounts

import (
	"net/http/httptest"
	"regexp"
	"testing"
)

// TestNormalizePhone: numbers with their country code become E.164, as the
// front end's normalizePhone makes them; the rest are refused.
func TestNormalizePhone(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		"+251 91 123 4567":   "+251911234567",
		"00251-911-234-567":  "+251911234567",
		" +254 (712) 345678": "+254712345678",
		"+1 202 555 0143":    "+12025550143",
		"0911234567":         "",
		"+0911234567":        "",
		"+12345":             "",
		"+1234567890123456":  "",
		"":                   "",
		"phone":              "",
	} {
		if got := NormalizePhone(raw); got != want {
			t.Errorf("%q: %q, want %q", raw, got, want)
		}
	}
}

// TestClientAddress: a trusted proxy's header gives the last address in
// it, the connection's address is used otherwise, and IPv6 addresses are
// keyed by their /64.
func TestClientAddress(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		remote, header, value, want string
	}{
		{"203.0.113.7:51234", "", "", "203.0.113.7"},
		{"203.0.113.7:51234", "X-Forwarded-For", "", "203.0.113.7"},
		{"10.0.0.2:80", "X-Forwarded-For", "198.51.100.1, 192.0.2.44", "192.0.2.44"},
		{"10.0.0.2:80", "", "192.0.2.44", "10.0.0.2"},
		{"[2001:db8:1:2:aaaa::1]:443", "", "", "2001:db8:1:2::/64"},
		{"10.0.0.2:80", "X-Forwarded-For", "2001:db8:1:2:ffff::9", "2001:db8:1:2::/64"},
		{"[::ffff:203.0.113.7]:51234", "", "", "203.0.113.7"},
	} {
		r := httptest.NewRequest("POST", "/api/auth/phone/start", nil)
		r.RemoteAddr = tc.remote
		if tc.value != "" {
			r.Header.Set("X-Forwarded-For", tc.value)
		}
		if got := clientAddress(r, tc.header); got != tc.want {
			t.Errorf("%s via %q %q: %s, want %s", tc.remote, tc.header, tc.value, got, tc.want)
		}
	}
}

func TestNewCode(t *testing.T) {
	t.Parallel()
	six := regexp.MustCompile(`^\d{6}$`)
	seen := map[string]bool{}
	for range 200 {
		c, err := newCode()
		if err != nil || !six.MatchString(c) {
			t.Fatalf("%q, %v", c, err)
		}
		seen[c] = true
	}
	if len(seen) < 190 {
		t.Fatalf("only %d distinct codes in 200", len(seen))
	}
}
