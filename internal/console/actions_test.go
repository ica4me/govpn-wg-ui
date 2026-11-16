package console

import (
	"context"
	"strings"
	"testing"
)

func TestActionDownloadOptions(t *testing.T) {
	for _, kind := range []string{"sites", "clients", "groups"} {
		t.Run(kind, func(t *testing.T) {
			var b strings.Builder
			_ = Resources(Page{Kind: kind, Rows: []Record{{"id": "office", "name": "Office"}}}).Render(context.Background(), &b)
			text := b.String()
			if !strings.Contains(text, ">Manage</a>") {
				t.Fatal("Manage changed")
			}
			for _, label := range []string{"↓ WireGuard .conf", "↓ Linux install script", "↓ Windows configuration"} {
				if strings.Contains(text, label) != (kind != "groups") {
					t.Fatalf("incorrect download option %s", label)
				}
			}
			if kind != "groups" {
				for _, path := range []string{"/" + kind + "/office/download/conf", "/" + kind + "/office/download/linux"} {
					if !strings.Contains(text, path) {
						t.Fatal("missing " + path)
					}
				}
			}
		})
	}
}

func TestResourceActionsAreClickPopover(t *testing.T) {
	var b strings.Builder
	p := Page{Kind: "sites", Rows: []Record{{"id": "office", "name": "Office"}}}
	if err := Resources(p).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`popovertarget="actions-sites-office"`, `popover="auto"`, `aria-label="Actions for Office"`, `⋮`} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("missing %s", want)
		}
	}
}
