package provenance

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/store"
)

type ExplainCodeOptions struct {
	Path   string
	Line   int
	Symbol string
	Commit string
	TaskID string
}

type ExplainCodePacket struct {
	Schema                 string           `json:"schema"`
	Project                string           `json:"project"`
	Entry                  ExplainCodeEntry `json:"entry"`
	Git                    ExplainGitFacts  `json:"git"`
	Tasks                  []ExplainTask    `json:"tasks,omitempty"`
	Facts                  []ExplainFact    `json:"facts"`
	Conflicts              []ExplainIssue   `json:"conflicts"`
	MissingProvenance      []ExplainIssue   `json:"missing_provenance"`
	MachineInferenceInputs []string         `json:"machine_inference_inputs"`
	Privacy                Privacy          `json:"privacy"`
	AuthorityBoundary      string           `json:"authority_boundary"`
}

type ExplainCodeEntry struct {
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Symbol string `json:"symbol,omitempty"`
	Commit string `json:"commit,omitempty"`
	TaskID string `json:"task_id,omitempty"`
}

type ExplainGitFacts struct {
	Commit       string   `json:"commit"`
	ShortCommit  string   `json:"short_commit"`
	AuthorDate   string   `json:"author_date,omitempty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
	PathExists   bool     `json:"path_exists,omitempty"`
	LineCommit   string   `json:"line_commit,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	SymbolKind   string   `json:"symbol_kind,omitempty"`
	SymbolLine   int      `json:"symbol_line,omitempty"`
}

type ExplainFact struct {
	Ref     string `json:"ref"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Summary string `json:"summary"`
}

type ExplainIssue struct {
	Ref     string `json:"ref"`
	Summary string `json:"summary"`
}

type ExplainTask struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Status        string            `json:"status"`
	Role          string            `json:"role"`
	ScopeMatches  []string          `json:"scope_matches,omitempty"`
	Contracts     []ExplainContract `json:"contracts,omitempty"`
	CanonicalRefs []string          `json:"canonical_refs,omitempty"`
	Decisions     []ExplainDecision `json:"decisions,omitempty"`
	EvidenceRefs  []EvidenceRef     `json:"evidence_refs,omitempty"`
	ReviewRefs    []ReviewRef       `json:"review_refs,omitempty"`
	CommitRefs    []string          `json:"commit_refs,omitempty"`
}

type ExplainContract struct {
	Ref  string `json:"ref"`
	Text string `json:"text"`
}

type ExplainDecision struct {
	Ref             string   `json:"ref"`
	QualityState    string   `json:"quality_state"`
	Current         bool     `json:"current"`
	Decision        string   `json:"decision"`
	Trigger         string   `json:"trigger"`
	Chosen          string   `json:"chosen"`
	Reason          string   `json:"reason"`
	Risk            string   `json:"risk"`
	ScopeAdded      []string `json:"scope_added,omitempty"`
	ValidationRefs  []string `json:"validation_refs,omitempty"`
	FactRefs        []string `json:"fact_refs,omitempty"`
	SupersedesID    int64    `json:"supersedes_id,omitempty"`
	SupersededByID  int64    `json:"superseded_by_id,omitempty"`
	AcceptanceBound bool     `json:"acceptance_required"`
}

func BuildExplainCode(ctx context.Context, cfg config.Config, configPath, root string, s *store.Store, opts ExplainCodeOptions) (ExplainCodePacket, error) {
	entryPath, err := explainRepoPath(root, opts.Path)
	if err != nil {
		return ExplainCodePacket{}, err
	}
	if entryPath == "" && strings.TrimSpace(opts.Commit) == "" && strings.TrimSpace(opts.TaskID) == "" {
		return ExplainCodePacket{}, fmt.Errorf("explain code requires a path, --commit, or --task")
	}
	if opts.Line < 0 {
		return ExplainCodePacket{}, fmt.Errorf("--line cannot be negative")
	}
	if opts.Line > 0 && entryPath == "" {
		return ExplainCodePacket{}, fmt.Errorf("--line requires a path")
	}
	if strings.TrimSpace(opts.Symbol) != "" && entryPath == "" {
		return ExplainCodePacket{}, fmt.Errorf("--symbol requires a path")
	}

	allTasks, err := s.AllTasks(ctx)
	if err != nil {
		return ExplainCodePacket{}, err
	}
	var explicitTask *store.Task
	if strings.TrimSpace(opts.TaskID) != "" {
		for i := range allTasks {
			if allTasks[i].Definition.ID == strings.TrimSpace(opts.TaskID) {
				explicitTask = &allTasks[i]
				break
			}
		}
		if explicitTask == nil {
			return ExplainCodePacket{}, store.ErrNotFound
		}
	}
	commitRef := strings.TrimSpace(opts.Commit)
	if commitRef == "" && explicitTask != nil && explicitTask.CommitSHA != "" {
		commitRef = explicitTask.CommitSHA
	}
	commit, err := fairwaygit.ResolveCommit(root, commitRef)
	if err != nil {
		return ExplainCodePacket{}, err
	}

	warnings := []string{}
	packet := ExplainCodePacket{
		Schema:            "fairway.explain-code.v1",
		Project:           cfg.Fairway.ProjectName,
		Entry:             ExplainCodeEntry{Path: entryPath, Line: opts.Line, Symbol: strings.TrimSpace(opts.Symbol), Commit: commit.SHA, TaskID: strings.TrimSpace(opts.TaskID)},
		Git:               ExplainGitFacts{Commit: commit.SHA, ShortCommit: commit.ShortSHA, AuthorDate: commit.AuthorDate, ChangedFiles: append([]string{}, commit.ChangedFiles...)},
		Facts:             []ExplainFact{},
		Conflicts:         []ExplainIssue{},
		MissingProvenance: []ExplainIssue{},
		Privacy:           defaultPrivacy(),
		AuthorityBoundary: "the packet reports recorded Fairway and Git facts plus bounded machine inputs; it does not generate historical rationale or grant approval, merge, deploy, release, credential, public-exposure, or live-operation authority",
	}
	packet.Facts = append(packet.Facts, ExplainFact{Ref: "git:commit:" + commit.SHA, Kind: "git_commit", State: "recorded", Summary: "resolved commit " + commit.SHA})

	var fileData []byte
	if entryPath != "" {
		fileData, err = fairwaygit.FileAtCommit(root, commit.SHA, entryPath)
		if err != nil {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "git:path:" + entryPath, Summary: err.Error()})
		} else {
			packet.Git.PathExists = true
			packet.Facts = append(packet.Facts, ExplainFact{Ref: "git:path:" + entryPath + "@" + commit.SHA, Kind: "git_path", State: "recorded", Summary: "path exists at selected commit"})
		}
	}
	if opts.Line > 0 && len(fileData) > 0 {
		lineCommit, blameErr := fairwaygit.BlameLine(root, commit.SHA, entryPath, opts.Line)
		if blameErr != nil {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: fmt.Sprintf("git:line:%s:%d", entryPath, opts.Line), Summary: blameErr.Error()})
		} else {
			packet.Git.LineCommit = lineCommit
			packet.Facts = append(packet.Facts, ExplainFact{Ref: fmt.Sprintf("git:line:%s:%d@%s", entryPath, opts.Line, commit.SHA), Kind: "git_line", State: "recorded", Summary: "line blame resolves to commit " + lineCommit})
		}
	}
	if packet.Entry.Symbol != "" && len(fileData) > 0 {
		kind, line, symbolErr := resolveCodeSymbol(entryPath, fileData, packet.Entry.Symbol)
		if symbolErr != nil {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "git:symbol:" + packet.Entry.Symbol, Summary: symbolErr.Error()})
		} else {
			packet.Git.Symbol = packet.Entry.Symbol
			packet.Git.SymbolKind = kind
			packet.Git.SymbolLine = line
			packet.Facts = append(packet.Facts, ExplainFact{Ref: "git:symbol:" + packet.Entry.Symbol + "@" + commit.SHA, Kind: "code_symbol", State: "recorded", Summary: fmt.Sprintf("%s resolves at line %d", kind, line)})
		}
	}

	candidates := explainCandidateTasks(allTasks, explicitTask, entryPath, commit)
	if len(candidates) == 0 {
		packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "fairway:task", Summary: "no Fairway task maps to the selected entry point"})
	}
	for _, candidate := range candidates {
		report, buildErr := Build(ctx, cfg, root, configPath, s, Options{TaskID: candidate.Definition.ID})
		if buildErr != nil {
			return ExplainCodePacket{}, buildErr
		}
		if len(report.Tasks) == 0 {
			continue
		}
		record := report.Tasks[0]
		warnings = append(warnings, report.Warnings...)
		explainTask := ExplainTask{ID: record.ID, Title: redactString(record.Title, &warnings, record.ID, "title"), Status: record.Status, Role: record.Role, EvidenceRefs: record.EvidenceRefs, ReviewRefs: record.ReviewRefs, CommitRefs: record.CommitRefs}
		explainTask.ScopeMatches = taskScopeMatches(candidate, entryPath, commit.ChangedFiles)
		for i, acceptance := range record.Acceptance {
			explainTask.Contracts = append(explainTask.Contracts, ExplainContract{Ref: fmt.Sprintf("task:%s:acceptance:%d", record.ID, i+1), Text: acceptance})
		}
		for _, ref := range append(append([]string{}, record.SourcePaths...), record.TargetPaths...) {
			if strings.HasSuffix(strings.ToLower(ref), ".md") {
				explainTask.CanonicalRefs = appendUnique(explainTask.CanonicalRefs, ref)
			}
		}
		decisions, decisionErr := s.TaskDecisions(ctx, record.ID)
		if decisionErr != nil {
			return ExplainCodePacket{}, decisionErr
		}
		for _, decision := range decisions {
			explainTask.Decisions = append(explainTask.Decisions, explainDecision(record.ID, decision, &warnings))
		}
		if len(decisions) == 0 {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "task:" + record.ID + ":decision", Summary: "no structured task decision is recorded"})
		}
		if len(record.EvidenceRefs) == 0 {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "task:" + record.ID + ":evidence", Summary: "no evidence reference is recorded"})
		}
		if len(record.ReviewRefs) == 0 {
			packet.MissingProvenance = append(packet.MissingProvenance, ExplainIssue{Ref: "task:" + record.ID + ":review", Summary: "no review reference is recorded"})
		}
		if explicitTask != nil && entryPath != "" && !taskMatchesPath(candidate, entryPath) {
			packet.Conflicts = append(packet.Conflicts, ExplainIssue{Ref: "task:" + record.ID + ":scope", Summary: "explicit task does not declare or independently accept the selected path"})
		}
		if strings.TrimSpace(opts.Commit) != "" && candidate.CommitSHA != "" && !commitMatches(candidate.CommitSHA, commit.SHA) {
			packet.Conflicts = append(packet.Conflicts, ExplainIssue{Ref: "task:" + record.ID + ":commit", Summary: "task commit differs from selected commit"})
		}
		packet.Tasks = append(packet.Tasks, explainTask)
		packet.Facts = append(packet.Facts, ExplainFact{Ref: "task:" + record.ID, Kind: "fairway_task", State: "recorded", Summary: fmt.Sprintf("task status=%s role=%s", record.Status, firstNonEmpty(record.Role, "unknown"))})
	}

	if len(warnings) > 0 {
		packet.Privacy.RedactionApplied = true
	}
	packet.MachineInferenceInputs = explainInferenceRefs(packet)
	sort.Slice(packet.Tasks, func(i, j int) bool { return packet.Tasks[i].ID < packet.Tasks[j].ID })
	sort.Slice(packet.Facts, func(i, j int) bool { return packet.Facts[i].Ref < packet.Facts[j].Ref })
	sort.Slice(packet.Conflicts, func(i, j int) bool { return packet.Conflicts[i].Ref < packet.Conflicts[j].Ref })
	sort.Slice(packet.MissingProvenance, func(i, j int) bool { return packet.MissingProvenance[i].Ref < packet.MissingProvenance[j].Ref })
	return packet, nil
}

func explainRepoPath(root, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	abs := value
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	rel, err := filepath.Rel(root, filepath.Clean(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay within the repository")
	}
	return filepath.ToSlash(rel), nil
}

func explainCandidateTasks(tasks []store.Task, explicit *store.Task, entryPath string, commit fairwaygit.CommitDetail) []store.Task {
	if explicit != nil {
		return []store.Task{*explicit}
	}
	var out []store.Task
	for _, task := range tasks {
		matched := task.CommitSHA != "" && commitMatches(task.CommitSHA, commit.SHA)
		if entryPath != "" && taskMatchesPath(task, entryPath) {
			matched = true
		}
		for _, changed := range commit.ChangedFiles {
			if taskMatchesPath(task, changed) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Definition.ID < out[j].Definition.ID })
	return out
}

func taskMatchesPath(task store.Task, repoPath string) bool {
	scopes := append(append([]string{}, task.Definition.SourcePaths...), task.Definition.TargetPaths...)
	return fairwaygit.PathMatchesScope(repoPath, scopes) != ""
}

func taskScopeMatches(task store.Task, entryPath string, changedFiles []string) []string {
	scopes := append(append([]string{}, task.Definition.SourcePaths...), task.Definition.TargetPaths...)
	seen := map[string]bool{}
	if entryPath != "" {
		if matched := fairwaygit.PathMatchesScope(entryPath, scopes); matched != "" {
			seen[matched] = true
		}
	}
	for _, changed := range changedFiles {
		if matched := fairwaygit.PathMatchesScope(changed, scopes); matched != "" {
			seen[matched] = true
		}
	}
	var out []string
	for matched := range seen {
		out = append(out, matched)
	}
	sort.Strings(out)
	return out
}

func commitMatches(recorded, resolved string) bool {
	recorded = strings.TrimSpace(recorded)
	resolved = strings.TrimSpace(resolved)
	return recorded != "" && resolved != "" && (strings.HasPrefix(resolved, recorded) || strings.HasPrefix(recorded, resolved))
}

func explainDecision(taskID string, decision store.TaskDecision, warnings *[]string) ExplainDecision {
	ref := fmt.Sprintf("decision:%s:%d", taskID, decision.ID)
	return ExplainDecision{
		Ref:             ref,
		QualityState:    decision.QualityState,
		Current:         decision.SupersededByID == 0,
		Decision:        redactString(decision.Decision, warnings, taskID, ref+":decision"),
		Trigger:         redactString(decision.Trigger, warnings, taskID, ref+":trigger"),
		Chosen:          redactString(decision.Chosen, warnings, taskID, ref+":chosen"),
		Reason:          redactString(decision.Reason, warnings, taskID, ref+":reason"),
		Risk:            redactString(decision.Risk, warnings, taskID, ref+":risk"),
		ScopeAdded:      redactStrings(decision.ScopeAdded, warnings, taskID, ref+":scope"),
		ValidationRefs:  redactStrings(decision.ValidationRefs, warnings, taskID, ref+":validation"),
		FactRefs:        redactStrings(decision.FactRefs, warnings, taskID, ref+":facts"),
		SupersedesID:    decision.SupersedesID,
		SupersededByID:  decision.SupersededByID,
		AcceptanceBound: decision.AcceptanceRequired,
	}
}

func explainInferenceRefs(packet ExplainCodePacket) []string {
	seen := map[string]bool{}
	for _, fact := range packet.Facts {
		seen[fact.Ref] = true
	}
	for _, task := range packet.Tasks {
		for _, contract := range task.Contracts {
			seen[contract.Ref] = true
		}
		for _, decision := range task.Decisions {
			seen[decision.Ref] = true
		}
		for _, evidence := range task.EvidenceRefs {
			seen[evidence.Ref] = true
		}
		for _, review := range task.ReviewRefs {
			seen[review.Ref] = true
		}
	}
	var out []string
	for ref := range seen {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

func resolveCodeSymbol(repoPath string, data []byte, symbol string) (string, int, error) {
	if !strings.HasSuffix(strings.ToLower(repoPath), ".go") {
		return "", 0, fmt.Errorf("symbol resolution currently supports committed Go source only")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, repoPath, data, 0)
	if err != nil {
		return "", 0, fmt.Errorf("cannot parse committed Go source")
	}
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			name := node.Name.Name
			if node.Recv != nil && len(node.Recv.List) > 0 {
				if receiver := receiverName(node.Recv.List[0].Type); receiver != "" {
					name = receiver + "." + name
				}
			}
			if symbol == node.Name.Name || symbol == name {
				kind := "function"
				if node.Recv != nil {
					kind = "method"
				}
				return kind, fset.Position(node.Pos()).Line, nil
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					if typed.Name.Name == symbol {
						return "type", fset.Position(typed.Pos()).Line, nil
					}
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						if name.Name == symbol {
							return strings.ToLower(node.Tok.String()), fset.Position(name.Pos()).Line, nil
						}
					}
				}
			}
		}
	}
	return "", 0, fmt.Errorf("symbol %q not found in committed source", symbol)
}

func receiverName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return receiverName(node.X)
	case *ast.IndexExpr:
		return receiverName(node.X)
	case *ast.IndexListExpr:
		return receiverName(node.X)
	default:
		return ""
	}
}
