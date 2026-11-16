package console

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBootstrapPasswordRequiredThenRelogin(t *testing.T) {
	changed := false
	calls := []string{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_at": time.Now().Add(time.Hour), "requires_password_change": !changed})
		case "/auth/password":
			changed = true
			w.WriteHeader(204)
		case "/system/status":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected backend request %s", r.URL.Path)
		}
	}))
	defer backend.Close()
	s, _ := New(backend.URL, false)
	login := func() (*http.Cookie, Session, *httptest.ResponseRecorder) {
		g := req(s, "GET", "/login", nil, nil)
		c := g.Result().Cookies()[0]
		_, v := s.get(&http.Request{Header: http.Header{"Cookie": {c.String()}}})
		w := req(s, "POST", "/login", url.Values{"csrf_token": {v.CSRF}, "username": {"admin"}, "password": {"admin"}}, c)
		c = w.Result().Cookies()[0]
		_, v = s.get(&http.Request{Header: http.Header{"Cookie": {c.String()}}})
		return c, v, w
	}
	c, v, w := login()
	if w.Header().Get("Location") != "/account/password" {
		t.Fatalf("bootstrap not redirected: %s", w.Header().Get("Location"))
	}
	before := len(calls)
	for _, p := range []string{"/", "/dashboard", "/sites", "/system", "/clients/alice/download/conf", "/login"} {
		w := req(s, "GET", p, nil, c)
		if w.Code != 303 || w.Header().Get("Location") != "/account/password" {
			t.Errorf("unrestricted GET %s: %d", p, w.Code)
		}
	}
	for _, p := range []string{"/sites/new", "/system/tune", "/clients/alice/delete", "/login"} {
		w := req(s, "POST", p, url.Values{"csrf_token": {v.CSRF}}, c)
		if w.Code != 303 || w.Header().Get("Location") != "/account/password" {
			t.Errorf("unrestricted POST %s: %d", p, w.Code)
		}
	}
	page := req(s, "GET", "/account/password", nil, c)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "must change your password") {
		t.Fatal("missing bootstrap guidance")
	}
	if len(calls) != before {
		t.Fatal("bootstrap operations reached backend", calls)
	}
	w = req(s, "POST", "/account/password", url.Values{"csrf_token": {v.CSRF}, "current_password": {"admin"}, "new_password": {"changed-secret"}, "confirm_password": {"changed-secret"}}, c)
	if !changed || w.Header().Get("Location") != "/login" {
		t.Fatal("password not changed")
	}
	if req(s, "GET", "/", nil, c).Header().Get("Location") != "/login" {
		t.Fatal("old session survived")
	}
	c, v, w = login()
	if w.Header().Get("Location") != "/" || req(s, "GET", "/", nil, c).Code != 200 {
		t.Fatal("relogin failed")
	}
	if req(s, "POST", "/logout", url.Values{"csrf_token": {v.CSRF}}, c).Header().Get("Location") != "/login" {
		t.Fatal("logout failed")
	}
}
