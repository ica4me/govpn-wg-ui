package console

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFRejectsQueryTokens(t *testing.T) {
	s, calls := fixture(t)
	c, csrf := auth(s)
	w := req(s, "POST", "/groups/new?csrf_token="+url.QueryEscape(csrf), url.Values{"name": {"test"}}, c)
	if w.Code != http.StatusForbidden || len(*calls) != 0 {
		t.Fatal("CSRF token in URL accepted", w.Code)
	}
}

func TestCSRFRejectsInvalidOrigins(t *testing.T) {
	s, calls := fixture(t)
	c, csrf := auth(s)
	for _, origin := range []string{"http://example.com", "ftp://example.com", "https://user@example.com", "https://example.com/path", "https://example.com?query", "https://example.com#fragment", "//example.com"} {
		t.Run(origin, func(t *testing.T) {
			r := httptest.NewRequest("POST", "https://example.com/groups/new", strings.NewReader(url.Values{"csrf_token": {csrf}, "name": {"test"}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", origin)
			r.AddCookie(c)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Error("invalid or cross-scheme Origin accepted", w.Code)
			}
		})
	}
	if len(*calls) != 0 {
		t.Fatal("invalid origin reached backend")
	}
}

func TestCSRFMatchesConfiguredHTTPSBehindProxy(t *testing.T) {
	s, _ := fixture(t)
	s.secure = true
	c, csrf := auth(s)
	r := httptest.NewRequest("POST", "http://example.com/logout", strings.NewReader(url.Values{"csrf_token": {csrf}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://example.com")
	r.Header.Set("X-Forwarded-Proto", "http") // untrusted forwarding headers must not choose the origin
	r.AddCookie(c)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatal("configured HTTPS origin rejected behind proxy", w.Code)
	}
}
