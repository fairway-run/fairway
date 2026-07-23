package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type resolvedPaths struct {
	project string
	root    string
	relRoot string
}

func resolvePaths(projectRoot, knowledgeRoot string, create bool) (resolvedPaths, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return resolvedPaths{}, errors.New("knowledge project root is required")
	}
	project, err := filepath.Abs(filepath.Clean(projectRoot))
	if err != nil {
		return resolvedPaths{}, errors.New("resolve knowledge project root")
	}
	info, err := os.Lstat(project)
	if err != nil {
		return resolvedPaths{}, errors.New("read knowledge project root")
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return resolvedPaths{}, errors.New("knowledge project root must be a regular directory")
	}
	realProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		return resolvedPaths{}, errors.New("resolve knowledge project root custody")
	}
	project = realProject

	relRoot := strings.TrimSpace(knowledgeRoot)
	if relRoot == "" {
		relRoot = DefaultRoot
	}
	if filepath.IsAbs(relRoot) {
		return resolvedPaths{}, errors.New("knowledge root must be project-relative")
	}
	relRoot = filepath.Clean(filepath.FromSlash(relRoot))
	if relRoot == "." || relRoot == ".." || strings.HasPrefix(relRoot, ".."+string(filepath.Separator)) {
		return resolvedPaths{}, errors.New("knowledge root must remain inside project root")
	}
	root := filepath.Join(project, relRoot)
	if err := validateExistingPathChain(project, root); err != nil {
		return resolvedPaths{}, err
	}
	if !create {
		rootInfo, err := os.Lstat(root)
		if err != nil {
			return resolvedPaths{}, errors.New("read knowledge root")
		}
		if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
			return resolvedPaths{}, errors.New("knowledge root must be a regular directory")
		}
	}
	return resolvedPaths{project: project, root: root, relRoot: filepath.ToSlash(relRoot)}, nil
}

func validateExistingPathChain(project, target string) error {
	rel, err := filepath.Rel(project, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("knowledge path escapes project root")
	}
	current := project
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return errors.New("inspect knowledge path")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("knowledge path contains symlink at %s", filepath.ToSlash(filepath.Join(filepath.Base(project), mustRel(project, current))))
		}
		if current != target && !info.IsDir() {
			return errors.New("knowledge path ancestor must be a directory")
		}
	}
	return nil
}

func mustRel(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return rel
}

func safeProjectFile(paths resolvedPaths, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" || filepath.IsAbs(rel) {
		return "", errors.New("source path must be project-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("source path escapes project root")
	}
	path := filepath.Join(paths.project, clean)
	if err := validateExistingPathChain(paths.project, path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("source path must name a regular file")
	}
	return path, nil
}
