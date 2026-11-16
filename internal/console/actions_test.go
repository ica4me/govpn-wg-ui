package console

import (
	"context"
	"strings"
	"testing"
)

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
