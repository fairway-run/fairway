package state

import (
	"fmt"
)

type Config struct {
	Allowed     []string
	Terminal    []string
	Transitions [][2]string
}

func ValidateTransition(cfg Config, from, to string, reopen bool) error {
	if !contains(cfg.Allowed, to) {
		return fmt.Errorf("state %q is not allowed", to)
	}
	if contains(cfg.Terminal, from) && !reopen {
		return fmt.Errorf("task is terminal in %q; use --reopen to move it", from)
	}
	if len(cfg.Transitions) == 0 {
		return nil
	}
	for _, transition := range cfg.Transitions {
		if (transition[0] == "*" || transition[0] == from) && transition[1] == to {
			return nil
		}
	}
	return fmt.Errorf("transition %q -> %q is not allowed", from, to)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
