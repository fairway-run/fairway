package git

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type CommitDetail struct {
	SHA          string   `json:"sha"`
	ShortSHA     string   `json:"short_sha"`
	AuthorDate   string   `json:"author_date,omitempty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

func ResolveCommit(root, ref string) (CommitDetail, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	sha, err := output(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return CommitDetail{}, fmt.Errorf("commit %q not found", ref)
	}
	metadata, err := output(root, "show", "-s", "--format=%h%x1f%aI", sha)
	if err != nil {
		return CommitDetail{}, err
	}
	parts := strings.SplitN(metadata, "\x1f", 2)
	detail := CommitDetail{SHA: sha}
	if len(parts) > 0 {
		detail.ShortSHA = parts[0]
	}
	if len(parts) == 2 {
		detail.AuthorDate = parts[1]
	}
	detail.ChangedFiles, err = changedFilesForCommit(root, sha)
	if err != nil {
		return CommitDetail{}, err
	}
	return detail, nil
}

func FileAtCommit(root, commit, repoPath string) ([]byte, error) {
	repoPath, err := safeRepoPath(repoPath)
	if err != nil {
		return nil, err
	}
	out, err := outputRaw(root, "show", commit+":"+repoPath)
	if err != nil {
		return nil, fmt.Errorf("path %q not found at commit %s", repoPath, shortCommit(commit))
	}
	return []byte(out), nil
}

func BlameLine(root, commit, repoPath string, line int) (string, error) {
	if line <= 0 {
		return "", fmt.Errorf("line must be positive")
	}
	repoPath, err := safeRepoPath(repoPath)
	if err != nil {
		return "", err
	}
	out, err := output(root, "blame", "--porcelain", "-L", strconv.Itoa(line)+","+strconv.Itoa(line), commit, "--", repoPath)
	if err != nil {
		return "", fmt.Errorf("line %d not found in %q at commit %s", line, repoPath, shortCommit(commit))
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("no blame fact for line %d", line)
	}
	return fields[0], nil
}

func safeRepoPath(value string) (string, error) {
	value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if value == "" || value == "." || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, ":") {
		return "", fmt.Errorf("path must be a relative repository path")
	}
	return value, nil
}

func shortCommit(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
