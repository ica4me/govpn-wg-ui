package console

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPasswordChangeValidationAndProtections(t *testing.T) {
	s, calls := fixture(t)
	c, csrf := auth(s)
	for _, test := range []struct {
		name, current, next, confirm, token string
		cookie                              *http.Cookie
		status                              int
	}{
		{"anonymous", "old", "password123", "password123", csrf, nil, 303},
		{"missing csrf", "old", "password123", "password123", "", c, 403},
		{"missing current", "", "password123", "password123", csrf, c, 200},
		{"short", "old", "short", "short", csrf, c, 200},
		{"too long in bytes", "old", strings.Repeat("界", 25), strings.Repeat("界", 25), csrf, c, 200},
		{"mismatch", "old", "password123", "password124", csrf, c, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := req(s, "POST", "/account/password", url.Values{"csrf_token": {test.token}, "current_password": {test.current}, "new_password": {test.next}, "confirm_password": {test.confirm}}, test.cookie)
			if w.Code != test.status {
				t.Fatal("unexpected status", w.Code)
			}
			if test.status == 200 && !strings.Contains(w.Body.String(), `role="alert"`) {
				t.Fatal("validation error missing")
			}
			if strings.Contains(w.Body.String(), `value="`+test.next+`"`) || strings.Contains(w.Body.String(), "backend-secret-token") {
				t.Fatal("secret in response")
			}
		})
	}
	if len(*calls) != 0 {
		t.Fatal("invalid password request reached backend")
	}
	if w := req(s, "PUT", "/account/password", nil, c); w.Code != 405 {
		t.Fatal("PUT permitted")
	}
}

func TestPasswordChangeBackendErrors(t *testing.T) {
	for _, status := range []int{400, 401, 422, 429, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"Password change failed"}`))
			}))
			defer b.Close()
			s, _ := New(b.URL, false)
			c, csrf := auth(s)
			w := req(s, "POST", "/account/password", url.Values{"csrf_token": {csrf}, "current_password": {"secret-current"}, "new_password": {"secret-next"}, "confirm_password": {"secret-next"}}, c)
			if status == 401 {
				if w.Code != 303 || req(s, "GET", "/", nil, c).Code != 303 {
					t.Fatal("401 did not invalidate session")
				}
			} else {
				if w.Code != 200 || !strings.Contains(w.Body.String(), "Password change failed") {
					t.Fatal("missing backend error")
				}
				if _, v := s.get(&http.Request{Header: http.Header{"Cookie": {c.String()}}}); v.Token == "" {
					t.Fatal("session lost on retryable error")
				}
			}
			for _, secret := range []string{"secret-current", "secret-next", "backend-secret-token"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("secret reflected")
				}
			}
		})
	}
}

func TestPasswordChangeSuccess(t *testing.T) {
	calls := 0
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v1/auth/password" || r.Header.Get("Authorization") != "Bearer backend-secret-token" {
			t.Error("incorrect password API request")
		}
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		if len(in) != 2 || in["current_password"] != " old password " || in["new_password"] != " new password " {
			t.Error("password payload changed or unexpected fields")
		}
		w.WriteHeader(204)
	}))
	defer b.Close()
	s, _ := New(b.URL+"/api/v1", true)
	c, csrf := auth(s)
	page := req(s, "GET", "/account/password", nil, c)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `autocomplete="new-password"`) || !strings.Contains(page.Body.String(), csrf) {
		t.Fatalf("missing password page: %d", page.Code)
	}
	if calls != 0 {
		t.Fatal("GET reached backend")
	}
	w := req(s, "POST", "/account/password", url.Values{"csrf_token": {csrf}, "current_password": {" old password "}, "new_password": {" new password "}, "confirm_password": {" new password "}}, c)
	if w.Code != 303 || w.Header().Get("Location") != "/login" || w.Header().Get("HX-Redirect") != "/login" {
		t.Fatalf("did not redirect to login: %d", w.Code)
	}
	if calls != 1 {
		t.Fatal("incorrect number of API calls", calls)
	}
	if len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].Value != "" {
		t.Fatal("cookie not cleared")
	}
	if got := req(s, "GET", "/", nil, c); got.Code != 303 {
		t.Fatal("session survived password change")
	}
}
