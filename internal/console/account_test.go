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

type credCall struct {
	path string
	body map[string]string
}

func accountFixture(t *testing.T) (*Server, *[]credCall) {
	t.Helper()
	calls := []credCall{}
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/login" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "backend-secret-token", "expires_at": time.Now().Add(time.Hour)})
			return
		}
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		calls = append(calls, credCall{r.URL.Path, in})
		switch r.URL.Path {
		case "/auth/password":
			w.WriteHeader(204)
		case "/auth/credentials":
			_, _ = w.Write([]byte(`{"username":"` + in["new_username"] + `","relogin_required":true}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(b.Close)
	s, err := New(b.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	return s, &calls
}

func TestAccountPageShowsEditableUsername(t *testing.T) {
	s, _ := accountFixture(t)
	c, _ := auth(s)
	w := req(s, "GET", "/account/password", nil, c)
	html := w.Body.String()
	if w.Code != 200 || !strings.Contains(html, `name="new_username" value="admin"`) {
		t.Fatal("username field missing or not prefilled with current user")
	}
	if !strings.Contains(html, "reset-admin") {
		t.Fatal("forgot-login guidance missing")
	}
}

func TestAccountChangeUsernameAndPasswordSignsOut(t *testing.T) {
	s, calls := accountFixture(t)
	c, csrf := auth(s)
	w := req(s, "POST", "/account/password", url.Values{"csrf_token": {csrf}, "current_password": {"old-pass"}, "new_username": {"boss"}, "new_password": {"brand-new-pass"}, "confirm_password": {"brand-new-pass"}}, c)
	if w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to login, got %d %s", w.Code, w.Header().Get("Location"))
	}
	if len(*calls) != 1 || (*calls)[0].path != "/auth/credentials" || (*calls)[0].body["new_username"] != "boss" || (*calls)[0].body["new_password"] != "brand-new-pass" {
		t.Fatalf("backend call = %+v", *calls)
	}
	if req(s, "GET", "/", nil, c).Code != 303 {
		t.Fatal("session survived credential change")
	}
}

func TestAccountChangeUsernameOnly(t *testing.T) {
	s, calls := accountFixture(t)
	c, csrf := auth(s)
	w := req(s, "POST", "/account/password", url.Values{"csrf_token": {csrf}, "current_password": {"old-pass"}, "new_username": {"boss"}}, c)
	if w.Code != 303 {
		t.Fatalf("username-only change: %d %s", w.Code, w.Body.String())
	}
	if len(*calls) != 1 || (*calls)[0].path != "/auth/credentials" {
		t.Fatalf("backend call = %+v", *calls)
	}
	if _, ok := (*calls)[0].body["new_password"]; ok {
		t.Fatal("blank password was sent")
	}
}

func TestAccountPasswordOnlyUsesPasswordEndpoint(t *testing.T) {
	s, calls := accountFixture(t)
	c, csrf := auth(s)
	w := req(s, "POST", "/account/password", url.Values{"csrf_token": {csrf}, "current_password": {"old-pass"}, "new_username": {"admin"}, "new_password": {"brand-new-pass"}, "confirm_password": {"brand-new-pass"}}, c)
	if w.Code != 303 || len(*calls) != 1 || (*calls)[0].path != "/auth/password" {
		t.Fatalf("password-only: %d %+v", w.Code, *calls)
	}
}

func TestAccountValidation(t *testing.T) {
	for name, form := range map[string]url.Values{
		"nothing changed": {"current_password": {"x"}, "new_username": {"admin"}},
		"bad username":    {"current_password": {"x"}, "new_username": {"bad name!"}},
		"short password":  {"current_password": {"x"}, "new_username": {"admin"}, "new_password": {"short"}, "confirm_password": {"short"}},
		"mismatch":        {"current_password": {"x"}, "new_username": {"boss"}, "new_password": {"brand-new-pass"}, "confirm_password": {"other-pass-1"}},
		"no current":      {"new_username": {"boss"}},
	} {
		t.Run(name, func(t *testing.T) {
			s, calls := accountFixture(t)
			c, csrf := auth(s)
			form.Set("csrf_token", csrf)
			w := req(s, "POST", "/account/password", form, c)
			if w.Code != 200 || !strings.Contains(w.Body.String(), `role="alert"`) {
				t.Fatalf("expected inline error, got %d", w.Code)
			}
			if len(*calls) != 0 {
				t.Fatal("invalid input reached backend")
			}
		})
	}
}

func TestRefreshButtonHasAnimationHooks(t *testing.T) {
	s, _ := fixture(t)
	c, _ := auth(s)
	for _, p := range []string{"/", "/sites", "/clients", "/groups", "/system"} {
		html := req(s, "GET", p, nil, c).Body.String()
		if !strings.Contains(html, `data-refresh`) || !strings.Contains(html, `class="refresh-icon"`) {
			t.Errorf("%s: refresh button lacks animation hooks", p)
		}
	}
	css, js := req(s, "GET", "/static/console.css", nil, nil).Body.String(), req(s, "GET", "/static/console.js", nil, nil).Body.String()
	for _, want := range []string{"govpn-spin", "#page-progress", "prefers-reduced-motion"} {
		if !strings.Contains(css, want) {
			t.Errorf("css missing %s", want)
		}
	}
	if !strings.Contains(js, "is-refreshing") || !strings.Contains(js, "pageshow") {
		t.Error("js missing refresh state handling")
	}
}
