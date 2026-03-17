package wabsignal

import (
	"strings"
	"testing"

	"github.com/derekurban/wabii-signal/internal/cfg"
)

func TestApplyProjectScopeDoesNotInjectRunIDByDefault(t *testing.T) {
	t.Parallel()

	project := &cfg.Project{
		PrimaryService: "aneu-mobile-local",
		CurrentRun: &cfg.RunState{
			ID: "run-123",
		},
	}
	project.EnsureDefaults()

	got := applyProjectScope("logs", "{}", project, false)

	if !strings.Contains(got, `service_name="aneu-mobile-local"`) {
		t.Fatalf("expected service scope in %q", got)
	}
	if strings.Contains(got, "wabsignal_run_id") {
		t.Fatalf("did not expect implicit run scope in %q", got)
	}
}

func TestApplyProjectScopePreservesNoProjectScope(t *testing.T) {
	t.Parallel()

	project := &cfg.Project{PrimaryService: "aneu-mobile-local"}
	project.EnsureDefaults()

	got := applyProjectScope("logs", "{}", project, true)
	if got != "{}" {
		t.Fatalf("expected raw expr to be preserved, got %q", got)
	}
}
