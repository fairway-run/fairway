package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Status struct {
	Root         string   `json:"root"`
	Branch       string   `json:"branch"`
	Base         string   `json:"base"`
	Dirty        bool     `json:"dirty"`
	Staged       bool     `json:"staged"`
	Untracked    bool     `json:"untracked"`
	Ahead        int      `json:"ahead"`
	Behind       int      `json:"behind"`
	BaseAncestor bool     `json:"base_ancestor"`
	ChangedFiles []string `json:"changed_files"`
}

func CurrentBranch(root string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func Check(root, base string) (Status, error) {
	repoRoot, err := output(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return Status{}, err
	}
	status := Status{Root: repoRoot, Branch: CurrentBranch(repoRoot), Base: base}
	porcelain, err := output(repoRoot, "status", "--porcelain")
	if err != nil {
		return Status{}, err
	}
	for _, line := range strings.Split(porcelain, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		status.Dirty = true
		if strings.HasPrefix(line, "??") {
			status.Untracked = true
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '?' {
			status.Staged = true
		}
		if len(line) > 3 {
			status.ChangedFiles = append(status.ChangedFiles, strings.TrimSpace(line[3:]))
		}
	}
	if base == "" {
		return status, nil
	}
	if err := run(repoRoot, "rev-parse", "--verify", base); err != nil {
		return Status{}, fmt.Errorf("base %q not found: %w", base, err)
	}
	status.BaseAncestor = run(repoRoot, "merge-base", "--is-ancestor", base, "HEAD") == nil
	counts, err := output(repoRoot, "rev-list", "--left-right", "--count", base+"...HEAD")
	if err != nil {
		return Status{}, err
	}
	fields := strings.Fields(counts)
	if len(fields) == 2 {
		behind, err := strconv.Atoi(fields[0])
		if err != nil {
			return Status{}, err
		}
		ahead, err := strconv.Atoi(fields[1])
		if err != nil {
			return Status{}, err
		}
		status.Behind = behind
		status.Ahead = ahead
	}
	return status, nil
}

func ChangedFiles(root, base string) ([]string, error) {
	if base == "" {
		return nil, nil
	}
	if err := run(root, "rev-parse", "--verify", base); err != nil {
		return nil, fmt.Errorf("base %q not found: %w", base, err)
	}
	out, err := output(root, "diff", "--name-only", base+"...HEAD")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func output(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func run(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Run()
}
