package state

import "testing"

func TestValidateTransition_TerminalRequiresReopen(t *testing.T) {
	cfg := Config{Allowed: []string{"todo", "done"}, Terminal: []string{"done"}}
	if err := ValidateTransition(cfg, "done", "todo", false); err == nil {
		t.Fatal("expected terminal transition error")
	}
	if err := ValidateTransition(cfg, "done", "todo", true); err != nil {
		t.Fatalf("reopen transition failed: %v", err)
	}
}

func TestValidateTransition_RejectsUnlistedTransition(t *testing.T) {
	cfg := Config{
		Allowed:     []string{"todo", "in_progress", "done"},
		Terminal:    []string{"done"},
		Transitions: [][2]string{{"todo", "in_progress"}},
	}
	if err := ValidateTransition(cfg, "todo", "done", false); err == nil {
		t.Fatal("expected transition error")
	}
}
