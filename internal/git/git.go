package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Status struct {
	Root                string   `json:"root"`
	Branch              string   `json:"branch"`
	Base                string   `json:"base"`
	Upstream            string   `json:"upstream,omitempty"`
	Dirty               bool     `json:"dirty"`
	Staged              bool     `json:"staged"`
	Untracked           bool     `json:"untracked"`
	Ahead               int      `json:"ahead"`
	Behind              int      `json:"behind"`
	Unpushed            int      `json:"unpushed"`
	Unpulled            int      `json:"unpulled"`
	HasUpstream         bool     `json:"has_upstream"`
	BaseAncestor        bool     `json:"base_ancestor"`
	ChangedFiles        []string `json:"changed_files"`
	TrackedChangedFiles []string `json:"tracked_changed_files,omitempty"`
	UntrackedFiles      []string `json:"untracked_files,omitempty"`
}

type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

type Commit struct {
	SHA          string   `json:"sha"`
	ShortSHA     string   `json:"short_sha"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	AuthorDate   string   `json:"author_date"`
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

func BranchExists(root, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	return run(root, "rev-parse", "--verify", branch) == nil
}

func BranchMerged(root, branch, base string) bool {
	branch = strings.TrimSpace(branch)
	base = strings.TrimSpace(base)
	if branch == "" || base == "" {
		return false
	}
	if !BranchExists(root, branch) || !BranchExists(root, base) {
		return false
	}
	return run(root, "merge-base", "--is-ancestor", branch, base) == nil
}

func RemoteBranchExists(root, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	return run(root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch) == nil
}

func DeleteRemoteBranch(root, remote, branch string) error {
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	return run(root, "push", remote, "--delete", branch)
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
	porcelain, err := outputRaw(repoRoot, "status", "--porcelain", "-uall")
	if err != nil {
		return Status{}, err
	}
	porcelain = strings.TrimSuffix(porcelain, "\n")
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
			path := strings.TrimSpace(line[3:])
			status.ChangedFiles = append(status.ChangedFiles, path)
			if strings.HasPrefix(line, "??") {
				status.UntrackedFiles = append(status.UntrackedFiles, path)
			} else {
				status.TrackedChangedFiles = append(status.TrackedChangedFiles, path)
			}
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

func CommitsSinceRef(root, sinceRef string) ([]Commit, error) {
	if strings.TrimSpace(sinceRef) == "" {
		return nil, fmt.Errorf("since ref is required")
	}
	if err := run(root, "rev-parse", "--verify", sinceRef); err != nil {
		return nil, fmt.Errorf("since ref %q not found: %w", sinceRef, err)
	}
	return commits(root, sinceRef+"..HEAD")
}

func CommitsSince(root string, since time.Time) ([]Commit, error) {
	if since.IsZero() {
		return nil, fmt.Errorf("since time is required")
	}
	return commits(root, "--since="+since.Format(time.RFC3339))
}

func commits(root string, rangeArg string) ([]Commit, error) {
	format := "%H%x1f%h%x1f%aI%x1f%s%x1f%b%x1e"
	out, err := output(root, "log", "--reverse", "--format="+format, rangeArg)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSuffix(out, "\x1e")
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var result []Commit
	for _, raw := range strings.Split(out, "\x1e") {
		raw = strings.TrimPrefix(raw, "\n")
		raw = strings.TrimSuffix(raw, "\n")
		parts := strings.SplitN(raw, "\x1f", 5)
		if len(parts) != 5 {
			continue
		}
		commit := Commit{
			SHA:        parts[0],
			ShortSHA:   parts[1],
			AuthorDate: parts[2],
			Subject:    parts[3],
			Body:       strings.TrimSpace(parts[4]),
		}
		files, err := changedFilesForCommit(root, commit.SHA)
		if err != nil {
			return nil, err
		}
		commit.ChangedFiles = files
		result = append(result, commit)
	}
	return result, nil
}

func changedFilesForCommit(root, sha string) ([]string, error) {
	out, err := output(root, "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
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
	out, err := outputRaw(root, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func outputRaw(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func run(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Run()
}
