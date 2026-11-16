package console

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoginConcurrencyCapBeforeBackend(t *testing.T) {
	var calls atomic.Int32
	entered := make(chan struct{}, loginConcurrency)
	unblock := make(chan struct{})
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		entered <- struct{}{}
		<-unblock
		w.WriteHeader(401)
	}))
	defer b.Close()
	defer close(unblock)
	s, _ := New(b.URL, false)
	v := Session{CSRF: random(), Expiry: time.Now().Add(time.Hour)}
	c := &http.Cookie{Name: "govpn_session", Value: s.put(v)}
	post := func(i int) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"csrf_token": {v.CSRF}}.Encode()))
		r.RemoteAddr = fmt.Sprintf("198.51.100.%d:123", i)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(c)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	for i := 1; i <= loginConcurrency; i++ {
		go post(i)
	}
	for i := 0; i < loginConcurrency; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second * 5):
			t.Fatal("backend not entered")
		}
	}
	w := post(99)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || calls.Load() != loginConcurrency {
		t.Fatalf("cap failed: %d calls=%d", w.Code, calls.Load())
	}
}

func TestLoginLimiterBoundedAndExpires(t *testing.T) {
	s, _ := New("http://localhost", false)
	now := time.Now()
	for i := 0; i < loginMaxClients; i++ {
		release, _ := s.logins.admit(fmt.Sprint(i), now)
		if release == nil {
			t.Fatal("unexpected rejection", i)
		}
		release()
	}
	if release, _ := s.logins.admit("new", now); release != nil {
		release()
		t.Fatal("capacity exceeded")
	}
	if len(s.logins.clients) != loginMaxClients {
		t.Fatal("unbounded state")
	}
	release, _ := s.logins.admit("new", now.Add(loginWindow))
	if release == nil {
		t.Fatal("expired state not reclaimed")
	}
	release()
	if len(s.logins.clients) != 1 {
		t.Fatal("expired entries remain")
	}
}

func TestLoginRateLimitBeforeBackend(t *testing.T) {
	s, calls := fixture(t)
	g := req(s, "GET", "/login", nil, nil)
	c := g.Result().Cookies()[0]
	_, v := s.get(&http.Request{Header: http.Header{"Cookie": {c.String()}}})
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"csrf_token": {v.CSRF}, "password": {"wrong"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("X-Forwarded-For", "203.0.113."+string(rune('1'+i)))
		r.AddCookie(c)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if i == 5 && (w.Code != 429 || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "Too many")) {
			t.Fatalf("expected friendly limit: %d %s", w.Code, w.Body.String())
		}
	}
	if len(*calls) != 5 {
		t.Fatalf("limited request forwarded: %v", *calls)
	}
}
