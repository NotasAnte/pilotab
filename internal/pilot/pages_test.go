package pilot

import (
	"reflect"
	"testing"
)

const listing = "## Pages\n1: about:blank\n2: Inbox (3) - me@x.com (https://mail.google.com/inbox)\n3: http://127.0.0.1:4000/ping.html [selected]\n## Console\n4: not a page"

func TestParsePagesReadsOnlyThePagesSection(t *testing.T) {
	want := []page{
		{ID: 1, URL: "about:blank"},
		{ID: 2, URL: "https://mail.google.com/inbox"},
		{ID: 3, URL: "http://127.0.0.1:4000/ping.html"},
	}
	if got := parsePages(listing); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePages() = %+v, want %+v", got, want)
	}
}

func TestFilterPagesHidesPagesNotOwned(t *testing.T) {
	got := filterPages(listing, func(id int) bool { return id == 3 })
	want := "## Pages\n3: http://127.0.0.1:4000/ping.html [selected]\n## Console\n4: not a page"
	if got != want {
		t.Fatalf("filterPages() = %q, want %q", got, want)
	}
}
