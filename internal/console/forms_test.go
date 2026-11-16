package console

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// recordFixture serves one client record so form rendering can be checked
// against the exact values the backend returned.
func recordFixture(t *testing.T, client string) *Server {
	t.Helper()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/login" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "backend-secret-token", "expires_at": time.Now().Add(time.Hour)})
			return
		}
		if r.Header.Get("Authorization") != "Bearer backend-secret-token" {
			http.Error(w, `{"error":"unauthorized"}`, 401)
			return
		}
		switch r.URL.Path {
		case "/clients":
			_, _ = io.WriteString(w, "["+client+"]")
		case "/clients/xccvme":
			_, _ = io.WriteString(w, client)
		case "/sites":
			_, _ = io.WriteString(w, `[{"id":"site-a","name":"Site-A","local_subnets":["192.168.10.0/24"]},{"id":"site-b","name":"Site-B","local_subnets":["192.168.20.0/24"]},{"id":"site-c","name":"Site-C","local_subnets":["192.168.30.0/24"]}]`)
		case "/sites/site-a":
			_, _ = io.WriteString(w, `{"id":"site-a","name":"Site-A","local_subnets":["192.168.10.0/24"],"allow_all_subnets":false,"exposed_subnets":["192.168.10.0/24"],"can_access_server_lan":true,"allowed_target_sites":["site-b"],"tunnel_mode":"split","port":3479,"mtu":1380,"enabled":true}`)
		case "/groups":
			_, _ = io.WriteString(w, `[{"id":"default","name":"default"}]`)
		default:
			_, _ = io.WriteString(w, `[]`)
		}
	}))
	t.Cleanup(b.Close)
	s, err := New(b.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func picker(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, `<div class="site-picker">`)
	if start < 0 {
		t.Fatal("site picker missing")
	}
	rest := html[start:]
	end := strings.Index(rest, "</fieldset>")
	if end < 0 {
		t.Fatal("site picker not closed")
	}
	return rest[:end]
}

// TestFormPreservesSelections is the regression for the report that a client
// saved with one destination site came back with every site checked and full
// tunnel selected: templ emitted checked="false"/selected="false", and HTML
// treats the presence of those attributes as true.
func TestFormPreservesSelections(t *testing.T) {
	cases := []struct {
		name   string
		record string
		mode   string
	}{
		{
			name:   "split single site",
			record: `{"id":"xccvme","username":"xccvme","group":"default","target_sites":["site-a"],"allow_server_lan":true,"tunnel_mode":"split","dns":["1.1.1.1"],"enabled":true}`,
			mode:   "split",
		},
		{
			name:   "full single site disabled LAN",
			record: `{"id":"xccvme","username":"xccvme","group":"default","target_sites":["site-b"],"allow_server_lan":false,"tunnel_mode":"full","dns":["1.1.1.1"],"enabled":false}`,
			mode:   "full",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := recordFixture(t, c.record)
			cookie, _ := auth(s)
			w := req(s, "GET", "/clients/xccvme", nil, cookie)
			if w.Code != 200 {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			html := w.Body.String()
			for _, bad := range []string{`checked="false"`, `selected="false"`, `required="false"`} {
				if strings.Contains(html, bad) {
					t.Errorf("form contains %s, which browsers treat as enabled", bad)
				}
			}
			p := picker(t, html)
			if checked := strings.Count(p, "checked"); checked != 1 {
				t.Errorf("checked site count = %d, want 1", checked)
			}
			for _, site := range []string{"site-a", "site-b"} {
				want := strings.Contains(c.record, `"`+site+`"`)
				got := strings.Contains(p, `value="`+site+`" checked`)
				if want != got {
					t.Errorf("%s checked = %v, want %v", site, got, want)
				}
			}
			if c.mode == "split" {
				if !strings.Contains(html, `<option value="split" selected>`) || strings.Contains(html, `<option value="full" selected`) {
					t.Error("split tunnel is not the selected option")
				}
			} else {
				if !strings.Contains(html, `<option value="full" selected>`) || strings.Contains(html, `<option value="split" selected`) {
					t.Error("full tunnel is not the selected option")
				}
			}
			if c.name == "split single site" && !strings.Contains(html, `name="allow_server_lan" checked`) {
				t.Error("allow_server_lan true is not checked")
			}
			if c.name == "full single site disabled LAN" && strings.Contains(html, `name="allow_server_lan" checked`) {
				t.Error("allow_server_lan false is checked")
			}
		})
	}
}

// TestServerLANInfoButton covers the requested help affordance next to the
// server LAN toggle.
func TestServerLANInfoButton(t *testing.T) {
	s := recordFixture(t, `{"id":"xccvme","username":"xccvme","group":"default","target_sites":["site-a"],"allow_server_lan":true,"tunnel_mode":"split","enabled":true}`)
	cookie, _ := auth(s)
	html := req(s, "GET", "/clients/xccvme", nil, cookie).Body.String()
	if !strings.Contains(html, `<details class="field-info">`) || !strings.Contains(html, "Another local subnet in server") {
		t.Fatal("missing info control for server LAN access")
	}
	i := strings.Index(html, `name="allow_server_lan"`)
	if i < 0 || !strings.Contains(html[i:], "Another local subnet in server") {
		t.Fatal("info text is not attached to the server LAN field")
	}
}

// TestNewClientDefaultsAreUsable ensures the create form starts with split
// tunnel and server LAN access enabled, matching backend defaults.
func TestNewClientDefaultsAreUsable(t *testing.T) {
	s := recordFixture(t, `{}`)
	cookie, _ := auth(s)
	html := req(s, "GET", "/clients/new", nil, cookie).Body.String()
	if !strings.Contains(html, `<option value="split" selected>`) || strings.Contains(html, `<option value="full" selected`) {
		t.Error("new client form does not default to split tunnel")
	}
	if !strings.Contains(html, `name="allow_server_lan" checked`) {
		t.Error("new client form does not default to server LAN access")
	}
	if !strings.Contains(html, `name="enabled" checked`) {
		t.Error("new client form does not default to enabled")
	}
	p := picker(t, html)
	if strings.Contains(p, "checked") {
		t.Error("new client form pre-selects destination sites")
	}
}

// TestSiteFormPreservesSelections covers the same attribute bug on the site
// form: only the chosen destination sites and booleans may render as active.
func TestSiteFormPreservesSelections(t *testing.T) {
	s := recordFixture(t, `{}`)
	cookie, _ := auth(s)
	w := req(s, "GET", "/sites/site-a", nil, cookie)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	html := w.Body.String()
	for _, bad := range []string{`checked="false"`, `selected="false"`, `required="false"`} {
		if strings.Contains(html, bad) {
			t.Errorf("site form contains %s", bad)
		}
	}
	p := picker(t, html)
	if got := strings.Count(p, "checked"); got != 1 || !strings.Contains(p, `value="site-b" checked`) {
		t.Errorf("destination sites checked=%d, want only site-b", got)
	}
	if !strings.Contains(html, `name="can_access_server_lan" checked`) {
		t.Error("can_access_server_lan true is not checked")
	}
	if strings.Contains(html, `name="allow_all_subnets" checked`) {
		t.Error("allow_all_subnets false is checked")
	}
	if !strings.Contains(html, `<option value="split" selected>`) {
		t.Error("split tunnel is not selected")
	}
	if !strings.Contains(html, `name="port" value="3479"`) || !strings.Contains(html, `name="mtu" value="1380"`) {
		t.Error("port or MTU not preserved")
	}
}
