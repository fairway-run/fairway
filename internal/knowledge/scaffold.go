package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

var scaffoldDirectories = []string{
	"architecture",
	"decisions",
	"environments",
	"incidents-and-lessons",
	"operations",
	"product-domains",
}

var scaffoldFiles = map[string]string{
	"README.md": `# Engineering Knowledge

Project-owned, source-grounded engineering synthesis. Start at [the index](index.md).
`,
	"index.md": `---
knowledge_version: 1
title: Engineering knowledge index
status: draft
owner: unassigned
last_verified: 1970-01-01
review_by: 1970-01-01
source_sha: "0000000"
sources: []
supersedes: []
---

# Engineering Knowledge Index

- [Current state](current-state.md)
- [Open questions](open-questions.md)
`,
	"current-state.md": `---
knowledge_version: 1
title: Current state
status: draft
owner: unassigned
last_verified: 1970-01-01
review_by: 1970-01-01
source_sha: "0000000"
sources: []
supersedes: []
---

# Current State
`,
	"open-questions.md": `---
knowledge_version: 1
title: Open questions
status: draft
owner: unassigned
last_verified: 1970-01-01
review_by: 1970-01-01
source_sha: "0000000"
sources: []
supersedes: []
---

# Open Questions
`,
	"log.md": "# Knowledge Log\n",
	DefaultSourceManifest: `knowledge_sources_version: 1
classes:
  project-file:
    kind: project_file
    authority: operational
    roots:
      - docs
      - doc/api
      - doc/architecture
      - doc/operations
      - doc/product
  fairway-decision:
    kind: fairway
    authority: operational
    fairway_kind: decision
    requires_store_validation: true
  fairway-evidence:
    kind: fairway
    authority: evidence
    fairway_kind: evidence
    requires_store_validation: true
`,
}

// Scaffold creates the conventional project-owned knowledge tree without
// overwriting existing files.
func Scaffold(opts ScaffoldOptions) (ScaffoldResult, error) {
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, true)
	if err != nil {
		return ScaffoldResult{}, err
	}
	if err := os.MkdirAll(paths.root, 0o755); err != nil {
		return ScaffoldResult{}, errors.New("create knowledge root")
	}
	if err := validateExistingPathChain(paths.project, paths.root); err != nil {
		return ScaffoldResult{}, err
	}
	for _, rel := range scaffoldDirectories {
		dir := filepath.Join(paths.root, rel)
		if err := os.Mkdir(dir, 0o755); err != nil && !os.IsExist(err) {
			return ScaffoldResult{}, fmt.Errorf("create knowledge directory %s", rel)
		}
		if err := validateExistingPathChain(paths.project, dir); err != nil {
			return ScaffoldResult{}, err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ScaffoldResult{}, fmt.Errorf("knowledge scaffold path %s is not a regular directory", rel)
		}
	}

	result := ScaffoldResult{Root: paths.relRoot}
	names := make([]string, 0, len(scaffoldFiles))
	for name := range scaffoldFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(paths.root, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return ScaffoldResult{}, fmt.Errorf("existing knowledge scaffold path %s is not a regular file", name)
			}
			result.Existing = append(result.Existing, name)
			continue
		}
		if err != nil {
			return ScaffoldResult{}, fmt.Errorf("create knowledge scaffold file %s", name)
		}
		if _, err := file.WriteString(scaffoldFiles[name]); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return ScaffoldResult{}, fmt.Errorf("write knowledge scaffold file %s", name)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return ScaffoldResult{}, fmt.Errorf("close knowledge scaffold file %s", name)
		}
		result.Created = append(result.Created, name)
	}
	return result, nil
}
