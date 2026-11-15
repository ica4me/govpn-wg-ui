package console

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Server, *[]string) {
	t.Helper()
	calls := []string{}
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/auth/login" {
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["password"] != "admin" {
				http.Error(w, `{"error":"invalid credentials"}`, 401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "backend-secret-token", "expires_at": time.Now().Add(time.Hour)})
			return
		}
		if r.Header.Get("Authorization") != "Bearer backend-secret-token" {
			http.Error(w, `{"error":"unauthorized"}`, 401)
			return
		}
		if strings.Contains(r.URL.Path, "/download/") {
			w.Header().Set("Content-Disposition", `attachment; filename="peer.conf"`)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "PrivateKey = test-only")
			return
		}
		switch {
		case r.Method == "DELETE":
			w.WriteHeader(204)
		case r.Method == "PUT" || r.Method == "POST":
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.URL.Path == "/system/status":
			_, _ = io.WriteString(w, `{"sites":2,"clients":3,"groups":1,"endpoint":"vpn.example.test"}`)
		case r.URL.Path == "/sites":
			_, _ = io.WriteString(w, `[{"id":"alpha","name":"Alpha","local_subnets":["192.168.20.0/24"]},{"id":"beta","name":"Beta"}]`)
		case r.URL.Path == "/groups":
			_, _ = io.WriteString(w, `[{"id":"default","name":"default"}]`)
		case r.URL.Path == "/clients":
			_, _ = io.WriteString(w, `[{"id":"alice","username":"alice","group":"default","target_sites":["alpha","beta"],"enabled":true}]`)
		case r.URL.Path == "/clients/alice":
			_, _ = io.WriteString(w, `{"id":"alice","username":"alice","group":"default","target_sites":["alpha","beta"],"enabled":true}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(b.Close)
	s, e := New(b.URL, false)
	if e != nil {
		t.Fatal(e)
	}
	return s, &calls
}
func req(s *Server, method, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func auth(s *Server) (*http.Cookie, string) {
	v := Session{Token: "backend-secret-token", CSRF: random(), User: "admin", Expiry: time.Now().Add(time.Hour)}
	return &http.Cookie{Name: "govpn_session", Value: s.put(v)}, v.CSRF
}
func TestLoginSessionAndLogout(t *testing.T) {
	s, _ := fixture(t)
	g := req(s, "GET", "/login", nil, nil)
	if g.Code != 200 {
		t.Fatal(g.Code)
	}
	c := g.Result().Cookies()[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	_, v := s.get(&http.Request{Header: http.Header{"Cookie": []string{c.String()}}})
	bad := req(s, "POST", "/login", url.Values{"username": {"admin"}, "password": {"admin"}}, c)
	if bad.Code != 403 {
		t.Fatal("login csrf accepted")
	}
	w := req(s, "POST", "/login", url.Values{"csrf_token": {v.CSRF}, "username": {"admin"}, "password": {"admin"}}, c)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	newCookie := w.Result().Cookies()[0]
	if newCookie.Value == c.Value || strings.Contains(newCookie.Value, "backend") {
		t.Fatal("session not rotated or token exposed")
	}
	if strings.Contains(w.Body.String(), "backend-secret") {
		t.Fatal("token leaked")
	}
	_, v = s.get(&http.Request{Header: http.Header{"Cookie": []string{newCookie.String()}}})
	out := req(s, "POST", "/logout", url.Values{"csrf_token": {v.CSRF}}, newCookie)
	if out.Code != 303 {
		t.Fatal(out.Code)
	}
	if got := req(s, "GET", "/", nil, newCookie); got.Code != 303 {
		t.Fatal("session survived logout")
	}
}
func TestAllPagesRenderAndNoTokenLeak(t *testing.T) {
	s, _ := fixture(t)
	c, _ := auth(s)
	for _, p := range []string{"/", "/dashboard", "/system", "/sites", "/groups", "/clients", "/sites/new", "/clients/new", "/groups/new", "/clients/alice"} {
		t.Run(p, func(t *testing.T) {
			w := req(s, "GET", p, nil, c)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "backend-secret-token") {
				t.Fatal("token leaked")
			}
			if !strings.Contains(w.Body.String(), "/static/htmx.min.js") {
				t.Fatal("missing local assets")
			}
		})
	}
}
func TestCSRFAndOrigin(t *testing.T) {
	s, calls := fixture(t)
	c, csrf := auth(s)
	for _, token := range []string{"", "wrong"} {
		w := req(s, "POST", "/groups/new", url.Values{"csrf_token": {token}, "name": {"new"}}, c)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("bad CSRF reached backend")
	}
	r := httptest.NewRequest("POST", "http://example.com/groups/new", strings.NewReader(url.Values{"csrf_token": {csrf}, "name": {"new"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://evil.test")
	r.AddCookie(c)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin accepted")
	}
}
func TestMultiSiteAndImmutableFields(t *testing.T) {
	form := url.Values{"target_sites": {"alpha", "beta"}, "username": {"ignored"}, "group": {"ignored"}, "enabled": {"on"}, "tunnel_mode": {"split"}}
	r := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	p, e := parseFields(r, "clients", true)
	if e != nil {
		t.Fatal(e)
	}
	if len(p["target_sites"].([]string)) != 2 {
		t.Fatal(p)
	}
	if _, ok := p["group"]; ok {
		t.Fatal("immutable group sent")
	}
	if _, ok := p["username"]; ok {
		t.Fatal("immutable username sent")
	}
	if p["allow_server_lan"] != false {
		t.Fatal("unchecked bool missing")
	}
}
func TestCRUDAndMaintenanceProxy(t *testing.T) {
	s, calls := fixture(t)
	c, csrf := auth(s)
	for _, p := range []string{"/sites/new", "/sites/alpha", "/groups/new", "/groups/default", "/clients/new", "/clients/alice", "/sites/alpha/delete", "/groups/default/delete", "/clients/alice/delete", "/system/tune", "/system/reconcile"} {
		w := req(s, "POST", p, url.Values{"csrf_token": {csrf}, "name": {"Name"}, "username": {"new"}, "group": {"default"}, "local_subnets": {"192.168.10.0/24"}, "tunnel_mode": {"split"}, "confirm": {"yes"}}, c)
		if w.Code != 303 && w.Code != 200 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	for _, want := range []string{"POST /sites", "PUT /sites/alpha", "DELETE /sites/alpha", "POST /groups", "PUT /groups/default", "DELETE /groups/default", "POST /clients", "PUT /clients/alice", "DELETE /clients/alice", "POST /system/tune", "POST /system/reconcile"} {
		found := false
		for _, x := range *calls {
			if x == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s", want)
		}
	}
}
func TestDownloadsAndSecurityHeaders(t *testing.T) {
	s, _ := fixture(t)
	c, _ := auth(s)
	for _, p := range []string{"/sites/alpha/download/linux", "/sites/alpha/download/windows", "/sites/alpha/download/conf", "/clients/alice/download/conf", "/clients/alice/download/linux", "/clients/alice/download/qr"} {
		w := req(s, "GET", p, nil, c)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Disposition") == "" {
			t.Fatalf("%s %d", p, w.Code)
		}
	}
	if w := req(s, "GET", "/clients/alice/download/windows", nil, c); w.Code != 404 {
		t.Fatal("unsupported download advertised")
	}
	if w := req(s, "GET", "/clients/alice/download/conf", nil, nil); w.Code != 303 {
		t.Fatal("unauthenticated download")
	}
}
func TestExpiryAndSecureCookie(t *testing.T) {
	s, _ := fixture(t)
	s.secure = true
	w := httptest.NewRecorder()
	s.cookie(w, "id", time.Now().Add(time.Hour))
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("secure flag missing")
	}
	id := s.put(Session{Token: "x", CSRF: "x", Expiry: time.Now().Add(-time.Minute)})
	if w := req(s, "GET", "/", nil, &http.Cookie{Name: "govpn_session", Value: id}); w.Code != 303 {
		t.Fatal("expired session accepted")
	}
}
func TestNumericValidation(t *testing.T) {
	for _, v := range []string{"abc", "1279", "9001"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"name": {"test"}, "mtu": {v}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		if _, e := parseFields(r, "groups", false); e == nil {
			t.Fatal("invalid mtu accepted", v)
		}
	}
}
func TestNoMaintenanceOnGET(t *testing.T) {
	s, calls := fixture(t)
	c, _ := auth(s)
	_ = req(s, "GET", "/system", nil, c)
	for _, x := range *calls {
		if strings.HasPrefix(x, "POST") {
			t.Fatal("GET triggered mutation")
		}
	}
}
