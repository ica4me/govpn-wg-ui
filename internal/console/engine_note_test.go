package console

import (
	"strings"
	"testing"
)

func TestDashboardEngineNote(t *testing.T) {
	s, _ := fixture(t)
	c, _ := auth(s)
	html := req(s, "GET", "/", nil, c).Body.String()
	for _, want := range []string{"/static/wireguard.svg", "WireGuard Engine", "https://github.com/ica4me/govpn-wg", "https://github.com/ica4me/govpn-wg-ui"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Index(html, "engine-note") > strings.Index(html, "Auto-refresh every 1 minute") {
		t.Fatal("note must precede monitoring")
	}
	if strings.Contains(req(s, "GET", "/sites", nil, c).Body.String(), "engine-note") {
		t.Fatal("dashboard note leaked onto sites")
	}
	logo := req(s, "GET", "/static/wireguard.svg", nil, nil)
	if logo.Code != 200 || !strings.Contains(logo.Body.String(), "<svg") {
		t.Fatal("local SVG not served")
	}
}
func TestWireGuardVersionSnapshot(t *testing.T) {
	t.Setenv("GOVPN_WIREGUARD_VERSION", "1.0.0")
	if wireGuardVersion() != "1.0.0" {
		t.Fatal("snapshot ignored")
	}
	t.Setenv("GOVPN_WIREGUARD_VERSION", "")
	if wireGuardVersion() != "version not configured" {
		t.Fatal("missing version must not be invented")
	}
}
