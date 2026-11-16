package console

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var assets embed.FS

type Session struct {
	Token, CSRF, User string
	Expiry            time.Time
}
type Server struct {
	base     string
	client   *http.Client
	secure   bool
	mu       sync.Mutex
	sessions map[string]Session
}

func New(base string, secure bool) (*Server, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid backend URL")
	}
	return &Server{base: strings.TrimRight(base, "/"), secure: secure, client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, sessions: map[string]Session{}}, nil
}
func random() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *Server) cookie(w http.ResponseWriter, id string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{Name: "govpn_session", Value: id, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode, Expires: exp})
}
func (s *Server) get(r *http.Request) (string, Session) {
	c, e := r.Cookie("govpn_session")
	if e != nil {
		return "", Session{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[c.Value]
	if !ok || time.Now().After(v.Expiry) {
		delete(s.sessions, c.Value)
		return "", Session{}
	}
	return c.Value, v
}
func (s *Server) put(v Session) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if time.Now().After(v.Expiry) {
			delete(s.sessions, k)
		}
	}
	if len(s.sessions) >= 10000 {
		for k := range s.sessions {
			delete(s.sessions, k)
			break
		}
	}
	id := random()
	s.sessions[id] = v
	return id
}
func (s *Server) remove(id string) { s.mu.Lock(); delete(s.sessions, id); s.mu.Unlock() }

type apiError struct {
	Code    int
	Message string
}

func (e *apiError) Error() string { return e.Message }
func (s *Server) request(ctx context.Context, token, method, path string, input any) ([]byte, http.Header, error) {
	var b io.Reader
	if input != nil {
		v, e := json.Marshal(input)
		if e != nil {
			return nil, nil, e
		}
		b = bytes.NewReader(v)
	}
	req, e := http.NewRequestWithContext(ctx, method, s.base+path, b)
	if e != nil {
		return nil, nil, e
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := s.client.Do(req)
	if e != nil {
		return nil, nil, errors.New("Backend unavailable or timed out. Check the API service and retry.")
	}
	defer res.Body.Close()
	body, e := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if e != nil {
		return nil, nil, errors.New("Could not read backend response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var v struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &v)
		if v.Error == "" {
			v.Error = http.StatusText(res.StatusCode)
		}
		return nil, res.Header, &apiError{res.StatusCode, v.Error}
	}
	return body, res.Header, nil
}
func (s *Server) fetch(r *http.Request, v Session, path string, out any) error {
	b, _, e := s.request(r.Context(), v.Token, "GET", path, nil)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return errors.New("Backend returned an invalid response")
	}
	return nil
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("/", s.route)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		mux.ServeHTTP(w, r)
	})
}
func render(w http.ResponseWriter, r *http.Request, p Page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") == "true" && r.URL.Query().Get("partial") == "1" {
		_ = Content(p).Render(r.Context(), w)
	} else {
		_ = Layout(p).Render(r.Context(), w)
	}
}
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	id, v := s.get(r)
	if r.URL.Path == "/login" {
		s.login(w, r, id, v)
		return
	}
	if v.Token == "" {
		w.Header().Set("HX-Redirect", "/login")
		http.Redirect(w, r, "/login", 303)
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if e := r.ParseForm(); e != nil {
			http.Error(w, "Invalid form", 400)
			return
		}
		if !s.csrfOK(r, v) {
			http.Error(w, "Invalid CSRF token or origin; reload the page", 403)
			return
		}
	}
	if r.URL.Path == "/logout" {
		if r.Method != "POST" {
			http.Error(w, "Use POST", 405)
			return
		}
		s.remove(id)
		s.cookie(w, "", time.Unix(1, 0))
		http.Redirect(w, r, "/login", 303)
		return
	}
	if r.URL.Path == "/account/password" {
		s.password(w, r, id, v)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	kind := parts[0]
	if kind == "" {
		kind = "dashboard"
	}
	if kind != "dashboard" && kind != "sites" && kind != "groups" && kind != "clients" && kind != "system" {
		http.NotFound(w, r)
		return
	}
	p := Page{Title: Title(kind), Kind: kind, CSRF: v.CSRF, User: v.User, Query: r.URL.Query().Get("q"), Notice: r.URL.Query().Get("notice")}
	var err error
	if len(parts) == 4 && parts[2] == "download" && r.Method == "GET" {
		s.download(w, r, v, kind, parts[1], parts[3])
		return
	}
	if r.Method == "POST" {
		s.mutate(w, r, v, p, parts)
		return
	}
	if kind == "dashboard" || kind == "system" {
		p.Status = Record{}
		err = s.fetch(r, v, "/system/status", &p.Status)
	} else {
		err = s.fetch(r, v, "/"+kind, &p.Rows)
		if err == nil && len(parts) > 1 {
			p.Form = true
			p.Edit = parts[1] != "new"
			p.ID = parts[1]
			p.Item = Defaults(kind)
			if p.Edit {
				found := false
				for _, row := range p.Rows {
					if row.Text("id") == p.ID {
						p.Item = row
						found = true
						break
					}
				}
				if !found {
					http.NotFound(w, r)
					return
				}
				if kind != "groups" {
					err = s.fetch(r, v, "/"+kind+"/"+url.PathEscape(p.ID), &p.Item)
				}
			}
			p.Fields = Fields(kind, p.Edit)
			if err == nil && kind != "groups" {
				err = s.fetch(r, v, "/sites", &p.Sites)
			}
			if err == nil && kind == "clients" {
				err = s.fetch(r, v, "/groups", &p.Groups)
			}
		}
		if p.Query != "" {
			rows := []Record{}
			for _, row := range p.Rows {
				if strings.Contains(strings.ToLower(row.JSON()), strings.ToLower(p.Query)) {
					rows = append(rows, row)
				}
			}
			p.Rows = rows
		}
	}
	if err != nil {
		if s.expired(w, r, id, err) {
			return
		}
		p.Error = err.Error()
	}
	render(w, r, p)
}
func (s *Server) csrfOK(r *http.Request, v Session) bool {
	// Never accept tokens in URLs, where browser history and proxy logs retain them.
	t := r.PostFormValue("csrf_token")
	if t == "" {
		t = r.Header.Get("X-CSRF-Token")
	}
	if subtle.ConstantTimeCompare([]byte(t), []byte(v.CSRF)) != 1 || v.CSRF == "" {
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, e := url.Parse(o)
		scheme := "http"
		// SecureCookies is configured at startup, unlike spoofable proxy headers.
		if s.secure || r.TLS != nil {
			scheme = "https"
		}
		if e != nil || u.Scheme != scheme || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return false
		}
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func (s *Server) expired(w http.ResponseWriter, r *http.Request, id string, e error) bool {
	var a *apiError
	if errors.As(e, &a) && a.Code == 401 {
		s.remove(id)
		s.cookie(w, "", time.Unix(1, 0))
		w.Header().Set("HX-Redirect", "/login")
		http.Redirect(w, r, "/login", 303)
		return true
	}
	return false
}
func (s *Server) login(w http.ResponseWriter, r *http.Request, id string, v Session) {
	if r.Method == "GET" {
		if v.Token != "" {
			http.Redirect(w, r, "/", 303)
			return
		}
		if id == "" {
			v = Session{CSRF: random(), Expiry: time.Now().Add(20 * time.Minute)}
			id = s.put(v)
			s.cookie(w, id, v.Expiry)
		}
		_ = Login(v.CSRF, "").Render(r.Context(), w)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if e := r.ParseForm(); e != nil || !s.csrfOK(r, v) {
		// A restart or expired pre-login session invalidates old forms. Reject
		// this submission, but provide a fresh session/form for a safe retry.
		if id == "" {
			v = Session{CSRF: random(), Expiry: time.Now().Add(20 * time.Minute)}
			s.cookie(w, s.put(v), v.Expiry)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = Login(v.CSRF, "Sesi halaman sudah berakhir. Silakan masuk kembali.").Render(r.Context(), w)
		return
	}
	b, _, e := s.request(r.Context(), "", "POST", "/auth/login", map[string]string{"username": r.FormValue("username"), "password": r.FormValue("password")})
	if e != nil {
		w.WriteHeader(401)
		_ = Login(v.CSRF, e.Error()).Render(r.Context(), w)
		return
	}
	var token struct {
		Token  string    `json:"access_token"`
		Expiry time.Time `json:"expires_at"`
	}
	if json.Unmarshal(b, &token) != nil || token.Token == "" || !token.Expiry.After(time.Now()) {
		http.Error(w, "Invalid backend login response", 502)
		return
	}
	s.remove(id)
	v = Session{Token: token.Token, CSRF: random(), User: r.FormValue("username"), Expiry: token.Expiry}
	s.cookie(w, s.put(v), v.Expiry)
	http.Redirect(w, r, "/", 303)
}
func parseFields(r *http.Request, kind string, edit bool) (Record, error) {
	p := Record{}
	for _, f := range Fields(kind, edit) {
		v := strings.TrimSpace(r.FormValue(f.Key))
		switch f.Kind {
		case "bool":
			p[f.Key] = v == "on"
		case "sites":
			a := r.Form[f.Key]
			if a == nil {
				a = []string{}
			}
			p[f.Key] = a
		case "list":
			a := strings.FieldsFunc(v, func(c rune) bool { return c == ',' || c == '\n' || c == '\r' })
			list := []string{}
			for _, x := range a {
				if x = strings.TrimSpace(x); x != "" {
					list = append(list, x)
				}
			}
			p[f.Key] = list
		case "number":
			if v != "" {
				n, e := strconv.Atoi(v)
				if e != nil {
					return nil, fmt.Errorf("%s must be a number", f.Label)
				}
				if (f.Key == "mtu" && (n < 1280 || n > 9000)) || (f.Key == "port" && (n < 1 || n > 65535)) {
					return nil, fmt.Errorf("%s is outside the permitted range", f.Label)
				}
				p[f.Key] = n
			}
		default:
			if f.Required && v == "" {
				return nil, fmt.Errorf("%s is required", f.Label)
			}
			p[f.Key] = v
		}
	}
	return p, nil
}
func (s *Server) mutate(w http.ResponseWriter, r *http.Request, v Session, p Page, parts []string) {
	kind := p.Kind
	path := "/" + kind
	method := "POST"
	var input any
	var e error
	if kind == "system" {
		if len(parts) != 2 || (parts[1] != "tune" && parts[1] != "reconcile") {
			http.NotFound(w, r)
			return
		}
		path += "/" + parts[1]
		if r.FormValue("confirm") != "yes" {
			http.Error(w, "Explicit confirmation required", 400)
			return
		}
	} else if kind == "dashboard" {
		http.Error(w, "Not allowed", 405)
		return
	} else {
		edit := len(parts) == 2 && parts[1] != "new"
		del := len(parts) == 3 && parts[2] == "delete"
		if len(parts) > 3 || len(parts) == 3 && !del {
			http.NotFound(w, r)
			return
		}
		if edit || del {
			path += "/" + url.PathEscape(parts[1])
			method = "PUT"
		}
		if del {
			if r.FormValue("confirm") != "yes" {
				http.Error(w, "Explicit confirmation required", 400)
				return
			}
			method = "DELETE"
		} else {
			input, e = parseFields(r, kind, edit)
		}
	}
	var b []byte
	if e == nil {
		b, _, e = s.request(r.Context(), v.Token, method, path, input)
	}
	if e != nil {
		sid, _ := s.get(r)
		if s.expired(w, r, sid, e) {
			return
		}
		p.Error = e.Error()
		p.Item = Record{}
		if x, ok := input.(Record); ok {
			p.Item = x
		}
		if kind != "system" && method != "DELETE" {
			p.Form = true
			p.Edit = method == "PUT"
			if p.Edit {
				p.ID = parts[1]
			}
			p.Fields = Fields(kind, p.Edit)
			_ = s.fetch(r, v, "/sites", &p.Sites)
			_ = s.fetch(r, v, "/groups", &p.Groups)
		}
		if kind == "system" {
			_ = s.fetch(r, v, "/system/status", &p.Status)
		}
		render(w, r, p)
		return
	}
	if kind == "system" {
		p.Notice = "Operation completed"
		var raw any
		if json.Unmarshal(b, &raw) == nil {
			pretty, _ := json.MarshalIndent(raw, "", "  ")
			p.Output = string(pretty)
		}
		_ = s.fetch(r, v, "/system/status", &p.Status)
		render(w, r, p)
		return
	}
	http.Redirect(w, r, "/"+kind+"?notice="+url.QueryEscape("Saved successfully. Re-download configuration after changing routing, ports, MTU or keys."), 303)
}
func (s *Server) download(w http.ResponseWriter, r *http.Request, v Session, kind, id, format string) {
	valid := kind == "sites" && (format == "conf" || format == "linux" || format == "windows") || kind == "clients" && (format == "conf" || format == "linux" || format == "qr")
	if !valid {
		http.NotFound(w, r)
		return
	}
	b, h, e := s.request(r.Context(), v.Token, "GET", "/"+kind+"/"+url.PathEscape(id)+"/download/"+format, nil)
	if e != nil {
		sid, _ := s.get(r)
		if s.expired(w, r, sid, e) {
			return
		}
		http.Error(w, e.Error(), 502)
		return
	}
	for _, k := range []string{"Content-Type", "Content-Disposition"} {
		w.Header().Set(k, h.Get(k))
	}
	_, _ = w.Write(b)
}
