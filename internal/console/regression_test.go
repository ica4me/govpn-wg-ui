package console

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFormRetentionAndClientSiteSameID(t *testing.T) {
	r := Record{"dns": []string{"1.1.1.1", "8.8.8.8"}, "target_sites": []string{"alpha"}}
	if r.Text("dns") != "1.1.1.1, 8.8.8.8" || !r.Has("target_sites", "alpha") {
		t.Fatal("failed form retention")
	}
	p := Page{Kind: "clients", ID: "alpha", Edit: true, Item: r, Fields: Fields("clients", true), Sites: []Record{{"id": "alpha", "name": "Same ID site"}}}
	var b bytes.Buffer
	if e := ResourceForm(p).Render(context.Background(), &b); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(b.String(), "Same ID site") {
		t.Fatal("client was prevented selecting same-ID site")
	}
}
func TestMutation401ClearsSession(t *testing.T) {
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, `{"error":"expired"}`, 401) }))
	defer b.Close()
	s, _ := New(b.URL, false)
	c, csrf := auth(s)
	w := req(s, "POST", "/groups/new", url.Values{"csrf_token": {csrf}, "name": {"new"}}, c)
	if w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("mutation 401 did not sign out", w.Code)
	}
}
func TestLocalAssets(t *testing.T) {
	s, _ := fixture(t)
	for _, p := range []string{"app.css", "console.css", "console.js", "htmx.min.js", "vpn-logo.png"} {
		w := req(s, "GET", "/static/"+p, nil, nil)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatal("missing asset", p)
		}
	}
}
