package util

import "testing"

func TestResolveGrafanaRangeSince(t *testing.T) {
	f, to, err := ResolveGrafanaRange("30m", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if f != "now-30m" || to != "now" {
		t.Fatalf("unexpected range %s %s", f, to)
	}
}

func TestResolveGrafanaRangeFromTo(t *testing.T) {
	_, _, err := ResolveGrafanaRange("", "2026-01-01T00:00:00Z", "2026-01-01T01:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
}
