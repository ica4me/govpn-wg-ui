package console

import (
	"os"
	"strings"
	"testing"
)

func TestGaugePresentationScale(t *testing.T) {
	for _, tc := range []struct {
		n          int
		scale, arc string
	}{{0, "100", "0.0000 100"}, {1, "100", "1.0000 100"}, {100, "100", "100.0000 100"}, {101, "1000", "10.1000 100"}, {1234, "10000", "12.3400 100"}} {
		s := Record{"sites": tc.n}
		if gaugeScale(s, "sites") != tc.scale || gaugeArc(s, "sites") != tc.arc {
			t.Errorf("count %d scale/arc incorrect", tc.n)
		}
	}
}
func TestDashboardRadialCounts(t *testing.T) {
	s, _ := fixture(t)
	c, _ := auth(s)
	html := req(s, "GET", "/", nil, c).Body.String()
	if strings.Count(html, `class="count-gauge"`) != 3 {
		t.Fatal("expected three radial gauges")
	}
	if !strings.Contains(html, `class="footer-brand"`) {
		t.Fatal("footer alignment missing")
	}
	if p := os.Getenv("GOVPN_PREVIEW_HTML"); p != "" {
		html = strings.ReplaceAll(html, "/static/", "http://127.0.0.1:18092/preview-static/")
		if err := os.WriteFile(p, []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
