package util

import "testing"

func TestInjectDefaultLabels(t *testing.T) {
	in := `{service="api"} |= "error"`
	out := InjectDefaultLabels(in, map[string]string{"cluster": "prod", "service": "ignored"})
	want := `{service="api", cluster="prod"} |= "error"`
	if out != want {
		t.Fatalf("expected %s, got %s", want, out)
	}
}

func TestExpandAlias(t *testing.T) {
	aliases := map[string]string{"errors": `{level="error"}`}
	if got := ExpandAlias("@errors", aliases); got != `{level="error"}` {
		t.Fatalf("unexpected alias expansion: %s", got)
	}
}
