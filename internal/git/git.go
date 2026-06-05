package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Status struct {
	Root         string   `json:"root"`
	Branch       string   `json:"branch"`
	Base         string   `json:"base"`
	Upstream     string   `json:"upstream,omitempty"`
	Dirty        bool     `json:"dirty"`
	Staged       bool     `json:"staged"`
	Untracked    bool     `json:"untracked"`
	Ahead        int      `json:"ahead"`
	Behind       int      `json:"behind"`
	Unpushed     int      `json:"unpushed"`
	Unpulled     int      `json:"unpulled"`
	HasUpstream  bool     `json:"has_upstream"`
	BaseAncestor bool     `json:"base_ancestor"`
	ChangedFiles []string `json:"changed_files"`
}

type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
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

func EnsureWorktree(root, base, branch, path string) error {
	if err := ensureBranch(root, base, branch); err != nil {
		return err
	}
	worktrees, err := Worktrees(root)
	if err != nil {
		return err
	}
	for _, wt := range worktrees {
		if wt.Path == path {
			return nil
		}
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("path already exists and is not a registered worktree: %s", path)
	}
	if err := os.MkdirAll(parent(path), 0o755); err != nil {
		return err
	}
	return run(root, "worktree", "add", path, branch)
}

func RemoveWorktree(root, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return run(root, args...)
}

func CheckoutReviewBranch(root, sourceBranch, reviewBranch string) error {
	status, err := Check(root, "")
	if err != nil {
		return err
	}
	if status.Dirty {
		return fmt.Errorf("worktree has uncommitted changes")
	}
	if err := run(root, "rev-parse", "--verify", sourceBranch); err != nil {
		return fmt.Errorf("source branch %q not found: %w", sourceBranch, err)
	}
	if err := run(root, "branch", "-f", reviewBranch, sourceBranch); err != nil {
		return err
	}
	return run(root, "checkout", reviewBranch)
}

func Worktrees(root string) ([]Worktree, error) {
	out, err := output(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var result []Worktree
	var current Worktree
	flush := func() {
		if current.Path != "" {
			result = append(result, current)
			current = Worktree{}
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	flush()
	return result, nil
}

func LastCommit(root string) string {
	out, err := output(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
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
		fillUpstreamStatus(repoRoot, &status)
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
	fillUpstreamStatus(repoRoot, &status)
	return status, nil
}

func fillUpstreamStatus(root string, status *Status) {
	upstream, err := output(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil || upstream == "" {
		return
	}
	status.Upstream = upstream
	status.HasUpstream = true
	counts, err := output(root, "rev-list", "--left-right", "--count", upstream+"...HEAD")
	if err != nil {
		return
	}
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return
	}
	unpulled, err := strconv.Atoi(fields[0])
	if err != nil {
		return
	}
	unpushed, err := strconv.Atoi(fields[1])
	if err != nil {
		return
	}
	status.Unpulled = unpulled
	status.Unpushed = unpushed
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

func ensureBranch(root, base, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if run(root, "rev-parse", "--verify", branch) == nil {
		return nil
	}
	if base == "" {
		return run(root, "branch", branch)
	}
	return run(root, "branch", branch, base)
}

func parent(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "."
	}
	return path[:idx]
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
