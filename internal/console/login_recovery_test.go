package console

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStaleLoginFormRecoversAfterRestart(t *testing.T) {
	s, _ := New("http://127.0.0.1:8080/api/v1", false)
	h := s.Handler()
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest("GET", "http://console/login", nil))
	// The browser retains an old cookie/form when the UI process restarts.
	s.sessions = map[string]Session{}
	f := url.Values{"csrf_token": {"old-token"}, "username": {"someone"}, "password": {"secret"}}
	req := httptest.NewRequest("POST", "http://console/login", strings.NewReader(f.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://console")
	for _, c := range first.Result().Cookies() {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<form") || !strings.Contains(body, "csrf_token") || len(w.Result().Cookies()) == 0 {
		t.Fatal("stale login must render a fresh protected form and session cookie")
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "someone") {
		t.Fatal("credentials retained")
	}
}

func TestLoginHasNoAccountHints(t *testing.T) {
	s, _ := New("http://127.0.0.1:8080/api/v1", false)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/login", nil))
	b := strings.ToLower(w.Body.String())
	for _, hint := range []string{`value="admin"`, "administrator", "credentials authenticate", "tokens stay"} {
		if strings.Contains(b, hint) {
			t.Fatalf("account hint present: %s", hint)
		}
	}
}
