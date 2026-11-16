package console

import (
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyConfiguration(t *testing.T) {
	s, _ := New("http://localhost", false)
	if err := s.SetTrustedProxies("10.0.0.2/32,2001:db8::2/128"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ peer, xff, want string }{
		{"10.0.0.2:123", "1.2.3.4, 198.51.100.1", "198.51.100.1"},
		{"10.0.0.2:123", "198.51.100.1, 10.0.0.2", "198.51.100.1"},
		{"198.51.100.2:123", "1.2.3.4", "198.51.100.2"},
		{"10.0.0.2:123", "garbage", "10.0.0.2"},
		{"[2001:db8::2]:123", "2001:db8::5", "2001:db8::5"},
	} {
		r := httptest.NewRequest("POST", "/login", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.xff)
		if got := s.clientIP(r); got != tc.want {
			t.Errorf("got %s want %s", got, tc.want)
		}
	}
	for _, bad := range []string{"*", "10.0.0.2", "0.0.0.0/0", "::/0", ",", "bad/32"} {
		if s.SetTrustedProxies(bad) == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
