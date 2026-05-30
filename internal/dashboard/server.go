package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

type Server struct {
	store     *store.Store
	roles     []string
	worktrees []WorktreeStatus
	csrfToken string
}

type WorktreeStatus struct {
	Role       string
	Branch     string
	Path       string
	Registered bool
	Exists     bool
	Dirty      bool
	LastCommit string
}

type Rollup struct {
	Done  int
	Total int
}

type ProjectStore struct {
	Name  string
	Path  string
	Store *store.Store
}

func New(s *store.Store, roles []string, worktrees []WorktreeStatus) *Server {
	return &Server{store: s, roles: roles, worktrees: worktrees, csrfToken: newCSRFToken()}
}

func NewMulti(projects []ProjectStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		type projectView struct {
			Name            string
			Path            string
			Tasks           []store.Task
			TaskCount       int
			SessionCount    int
			CheckpointCount int
			WatcherCount    int
			Error           string
		}
		var views []projectView
		for _, project := range projects {
			view := projectView{Name: project.Name, Path: project.Path}
			tasks, err := project.Store.AllTasks(r.Context())
			if err != nil {
				view.Error = err.Error()
				views = append(views, view)
				continue
			}
			sessions, _ := project.Store.Sessions(r.Context(), false)
			checkpoints, _ := project.Store.Checkpoints(r.Context(), "", false)
			watchers, _ := project.Store.Watchers(r.Context(), false)
			view.Tasks = tasks
			view.TaskCount = len(tasks)
			view.SessionCount = len(sessions)
			view.CheckpointCount = len(checkpoints)
			view.WatcherCount = len(watchers)
			views = append(views, view)
		}
		_ = multiTemplate.Execute(w, struct{ Projects []projectView }{views})
	})
	return mux
}

type RoleGroup struct {
	Role    string
	Current *store.Task
	Tasks   []store.Task
}

type WorkstreamGroup struct {
	Label string
	Tasks []store.Task
}

type TaskFilters struct {
	Profile      string
	Kind         string
	OwningDomain string
	RiskLevel    string
	ReviewDomain string
}

type FilterOptions struct {
	Profiles      []string
	Kinds         []string
	OwningDomains []string
	RiskLevels    []string
	ReviewDomains []string
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/tasks/", s.task)
	mux.HandleFunc("/actions/claim", s.claim)
	mux.HandleFunc("/events", s.events)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activity, err := s.store.Activity(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	health, err := s.store.Health(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sessions, err := s.store.Sessions(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	checkpoints, err := s.store.Checkpoints(r.Context(), "", false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	staleCheckpoints, err := s.store.Checkpoints(r.Context(), time.Now().UTC().Format("2006-01-02"), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	watchers, err := s.store.Watchers(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filters := taskFiltersFromRequest(r)
	displayTasks := filterTasks(tasks, filters)
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	data := struct {
		Groups           []RoleGroup
		Workstreams      []WorkstreamGroup
		Filters          TaskFilters
		FilterOptions    FilterOptions
		Activity         []store.Activity
		Health           store.Health
		Sessions         []store.Session
		Worktrees        []WorktreeStatus
		Checkpoints      []store.Checkpoint
		StaleCheckpoints []store.Checkpoint
		Watchers         []store.Watcher
		Rollups          map[string]Rollup
	}{groupTasks(displayTasks, s.roles), groupWorkstreams(displayTasks), filters, filterOptions(tasks), activity, health, sessions, s.worktrees, checkpoints, staleCheckpoints, watchers, rollups}
	_ = indexTemplate.Execute(w, data)
}

func groupTasks(tasks []store.Task, roles []string) []RoleGroup {
	byRole := map[string]int{}
	var groups []RoleGroup
	addRole := func(role string) int {
		if role == "" {
			role = "unassigned"
		}
		if index, ok := byRole[role]; ok {
			return index
		}
		groups = append(groups, RoleGroup{Role: role})
		index := len(groups) - 1
		byRole[role] = index
		return index
	}
	for _, role := range roles {
		addRole(role)
	}
	for _, task := range tasks {
		index := addRole(task.Definition.Role)
		groups[index].Tasks = append(groups[index].Tasks, task)
		if task.Status == "in_progress" && groups[index].Current == nil {
			copy := task
			groups[index].Current = &copy
		}
	}
	return groups
}

func taskFiltersFromRequest(r *http.Request) TaskFilters {
	query := r.URL.Query()
	return TaskFilters{
		Profile:      strings.TrimSpace(query.Get("profile")),
		Kind:         strings.TrimSpace(query.Get("kind")),
		OwningDomain: strings.TrimSpace(query.Get("owning_domain")),
		RiskLevel:    strings.TrimSpace(query.Get("risk_level")),
		ReviewDomain: strings.TrimSpace(query.Get("review_domain")),
	}
}

func filterTasks(tasks []store.Task, filters TaskFilters) []store.Task {
	var out []store.Task
	for _, task := range tasks {
		if filters.Profile != "" && task.Definition.Profile != filters.Profile {
			continue
		}
		if filters.Kind != "" && task.Definition.Kind != filters.Kind {
			continue
		}
		if filters.OwningDomain != "" && task.Definition.OwningDomain != filters.OwningDomain {
			continue
		}
		if filters.RiskLevel != "" && task.Definition.RiskLevel != filters.RiskLevel {
			continue
		}
		if filters.ReviewDomain != "" && !containsString(task.Definition.ReviewDomains, filters.ReviewDomain) {
			continue
		}
		out = append(out, task)
	}
	return out
}

func filterOptions(tasks []store.Task) FilterOptions {
	var options FilterOptions
	profiles := map[string]bool{}
	kinds := map[string]bool{}
	domains := map[string]bool{}
	risks := map[string]bool{}
	reviewDomains := map[string]bool{}
	for _, task := range tasks {
		addFilterValue(profiles, task.Definition.Profile)
		addFilterValue(kinds, task.Definition.Kind)
		addFilterValue(domains, task.Definition.OwningDomain)
		addFilterValue(risks, task.Definition.RiskLevel)
		for _, domain := range task.Definition.ReviewDomains {
			addFilterValue(reviewDomains, domain)
		}
	}
	options.Profiles = sortedFilterValues(profiles)
	options.Kinds = sortedFilterValues(kinds)
	options.OwningDomains = sortedFilterValues(domains)
	options.RiskLevels = sortedFilterValues(risks)
	options.ReviewDomains = sortedFilterValues(reviewDomains)
	return options
}

func addFilterValue(values map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values[value] = true
	}
}

func sortedFilterValues(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func groupWorkstreams(tasks []store.Task) []WorkstreamGroup {
	byLabel := map[string]int{}
	var groups []WorkstreamGroup
	for _, task := range tasks {
		label := workstreamLabel(task)
		if label == "" {
			continue
		}
		index, ok := byLabel[label]
		if !ok {
			groups = append(groups, WorkstreamGroup{Label: label})
			index = len(groups) - 1
			byLabel[label] = index
		}
		groups[index].Tasks = append(groups[index].Tasks, task)
	}
	return groups
}

func workstreamLabel(task store.Task) string {
	profile := task.Definition.Profile
	kind := task.Definition.Kind
	switch {
	case profile != "" && kind != "":
		return profile + " / " + kind
	case profile != "":
		return profile
	case kind != "" && kind != "task":
		return kind
	default:
		return ""
	}
}

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/tasks/"):]
	task, transitions, evidence, handoffs, reviews, err := s.store.TaskDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	data := struct {
		Task        store.Task
		Transitions []store.Transition
		Evidence    []store.Evidence
		Handoffs    []store.Handoff
		Reviews     []store.Review
		Rollup      Rollup
		CSRFToken   string
	}{task, transitions, evidence, handoffs, reviews, rollups[task.Definition.ID], s.csrfToken}
	_ = detailTemplate.Execute(w, data)
}

func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.FormValue("csrf") != s.csrfToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	taskID := r.FormValue("task_id")
	task, _, _, _, _, err := s.store.TaskDetail(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := s.store.Claim(r.Context(), taskID, task.Definition.Role, ""); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.claim", TaskID: taskID, Detail: "claimed from dashboard"})
	http.Redirect(w, r, "/tasks/"+taskID, http.StatusSeeOther)
}

func taskRollups(tasks []store.Task, terminal map[string]bool) map[string]Rollup {
	parent := map[string]string{}
	status := map[string]string{}
	for _, task := range tasks {
		parent[task.Definition.ID] = task.Definition.ParentID
		status[task.Definition.ID] = task.Status
	}
	rollups := map[string]Rollup{}
	for _, task := range tasks {
		for cursor := parent[task.Definition.ID]; cursor != ""; cursor = parent[cursor] {
			rollup := rollups[cursor]
			rollup.Total++
			if terminal[status[task.Definition.ID]] {
				rollup.Done++
			}
			rollups[cursor] = rollup
		}
	}
	return rollups
}

func newCSRFToken() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes[:])
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	last, err := s.store.LatestHistoryID(r.Context())
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			current, err := s.store.LatestHistoryID(r.Context())
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
				flusher.Flush()
				return
			}
			if current > last {
				last = current
				fmt.Fprintf(w, "event: refresh\ndata: %d\n\n", current)
				flusher.Flush()
			}
		}
	}
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html><head><title>fairway</title><style>
body{font-family:system-ui,sans-serif;margin:32px;background:#f7f7f5;color:#1f2933}
table{border-collapse:collapse;width:100%;background:white;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left;vertical-align:top}
.status{font-family:monospace}.role{font-weight:600}.badges,.lanes,.filters{display:flex;gap:8px;margin:16px 0;flex-wrap:wrap}.badge,.lane{background:#fff;border:1px solid #ddd;padding:6px 8px;border-radius:6px}.lane b{display:block}.filters{background:white;border:1px solid #ddd;padding:8px}.filters label{display:grid;gap:4px;font-size:12px;color:#667085}.filters select{min-width:140px}.layout{display:grid;grid-template-columns:1fr 380px;gap:24px}.panel{background:white;border:1px solid #ddd;padding:12px;margin-bottom:16px}.panel p{border-bottom:1px solid #eee;padding-bottom:8px}.muted{color:#667085}.bad{color:#b42318}.ok{color:#027a48}
</style><script>
const events = new EventSource("/events");
events.addEventListener("refresh", () => window.location.reload());
</script></head><body>
<h1>fairway</h1>
<div class="badges">
<span class="badge">in progress: {{.Health.InProgress}}</span>
<span class="badge">stale claims: {{.Health.StaleInProgress}}</span>
<span class="badge">blocked &gt;24h: {{.Health.BlockedOver24h}}</span>
<span class="badge">handoffs &gt;1h: {{.Health.UnacknowledgedOver1Hour}}</span>
<span class="badge">reviews: {{.Health.UnroutedReviews}}</span>
<span class="badge">stale checkpoints: {{len .StaleCheckpoints}}</span>
<span class="badge">active watchers: {{len .Watchers}}</span>
<span class="badge">sessions: {{len .Sessions}}</span>
</div>
<form class="filters" method="get">
<label>Profile<select name="profile"><option value="">all</option>{{range .FilterOptions.Profiles}}<option value="{{.}}" {{if eq $.Filters.Profile .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Kind<select name="kind"><option value="">all</option>{{range .FilterOptions.Kinds}}<option value="{{.}}" {{if eq $.Filters.Kind .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Domain<select name="owning_domain"><option value="">all</option>{{range .FilterOptions.OwningDomains}}<option value="{{.}}" {{if eq $.Filters.OwningDomain .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Risk<select name="risk_level"><option value="">all</option>{{range .FilterOptions.RiskLevels}}<option value="{{.}}" {{if eq $.Filters.RiskLevel .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Review<select name="review_domain"><option value="">all</option>{{range .FilterOptions.ReviewDomains}}<option value="{{.}}" {{if eq $.Filters.ReviewDomain .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<button type="submit">Apply</button><a href="/">Clear</a>
</form>
<div class="lanes">
{{range .Groups}}<div class="lane"><b>{{.Role}}</b>{{if .Current}}<a href="/tasks/{{.Current.Definition.ID}}">{{.Current.Definition.ID}}</a> {{.Current.Definition.Title}}{{else}}idle{{end}}</div>{{end}}
</div>
<div class="layout">
<main>
<h2>Sessions</h2>
<table><tr><th>ID</th><th>Role</th><th>Status</th><th>Branch</th><th>Task</th><th>Backend</th></tr>
{{range .Sessions}}<tr><td>{{.ID}}</td><td class="role">{{.Role}}</td><td class="status">{{.Status}}</td><td>{{.Branch}}</td><td>{{if .TaskID}}<a href="/tasks/{{.TaskID}}">{{.TaskID}}</a>{{end}}</td><td>{{.SessionBackend}} {{.Provider}}</td></tr>{{else}}<tr><td colspan="6">no live sessions</td></tr>{{end}}
</table>
<h2>Worktrees</h2>
<table><tr><th>Role</th><th>Branch</th><th>State</th><th>Commit</th><th>Path</th></tr>
{{range .Worktrees}}<tr><td class="role">{{.Role}}</td><td>{{.Branch}}</td><td>{{if .Dirty}}<span class="bad">dirty</span>{{else}}<span class="ok">clean</span>{{end}} {{if not .Exists}}missing{{else if not .Registered}}unregistered{{end}}</td><td>{{.LastCommit}}</td><td class="muted">{{.Path}}</td></tr>{{else}}<tr><td colspan="5">no configured worktrees</td></tr>{{end}}
</table>
{{if .Workstreams}}
<h2>Workstreams</h2>
{{range .Workstreams}}
<h3>{{.Label}}</h3>
<table><tr><th>ID</th><th>Title</th><th>Role</th><th>Status</th><th>Domain</th><th>Risk</th><th>Review domains</th></tr>
{{range .Tasks}}<tr><td><a href="/tasks/{{.Definition.ID}}">{{.Definition.ID}}</a></td><td>{{.Definition.Title}}</td><td class="role">{{.Definition.Role}}</td><td class="status">{{.Status}}</td><td>{{.Definition.OwningDomain}}</td><td>{{.Definition.RiskLevel}}</td><td>{{range .Definition.ReviewDomains}}<code>{{.}}</code> {{end}}</td></tr>{{else}}<tr><td colspan="7">no tasks</td></tr>{{end}}
</table>
{{end}}
{{end}}
{{range .Groups}}
<h2>{{.Role}}</h2>
<table><tr><th>ID</th><th>Title</th><th>Kind</th><th>Profile</th><th>Status</th><th>Owner</th><th>Review</th><th>Rollup</th></tr>
{{range .Tasks}}<tr><td><a href="/tasks/{{.Definition.ID}}">{{.Definition.ID}}</a></td><td>{{.Definition.Title}}</td><td>{{.Definition.Kind}}</td><td>{{.Definition.Profile}}</td><td class="status">{{.Status}}</td><td>{{.Owner}}</td><td>{{.ReviewStatus}}</td><td>{{with index $.Rollups .Definition.ID}}{{.Done}}/{{.Total}}{{else}}-{{end}}</td></tr>{{else}}<tr><td colspan="8">no tasks</td></tr>{{end}}
</table>
{{end}}
</main>
<aside>
<section class="panel"><h2>Watchers</h2>{{range .Watchers}}<p><b>{{.ID}}</b> <a href="/tasks/{{.TaskID}}">{{.TaskID}}</a><br><code>{{.Status}}</code> {{.Owner}} {{.Process}}<br><small>{{.Command}}</small></p>{{else}}<p>none</p>{{end}}</section>
<section class="panel"><h2>Checkpoints</h2>{{range .Checkpoints}}<p><b><a href="/tasks/{{.TaskID}}">{{.TaskID}}</a></b> <code>{{.State}}</code><br>{{.Summary}}<br><small>{{.Owner}} {{.TargetCloseBy}}</small></p>{{else}}<p>none</p>{{end}}</section>
<section class="panel"><h2>Activity</h2>{{range .Activity}}<p><b>{{.TaskID}}</b> <code>{{.Kind}}</code><br>{{.Summary}}<br><small>{{.CreatedAt}} {{.Actor}}</small></p>{{else}}<p>none</p>{{end}}</section>
</aside>
</div>
</body></html>`))

var detailTemplate = template.Must(template.New("detail").Parse(`<!doctype html>
<html><head><title>{{.Task.Definition.ID}}</title><style>
body{font-family:system-ui,sans-serif;margin:32px;max-width:960px}.meta{color:#555}pre{background:#f4f4f4;padding:12px}table{border-collapse:collapse;width:100%;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left;vertical-align:top}code{background:#f4f4f4;padding:1px 3px}
</style></head><body>
<p><a href="/">back</a></p>
<h1>{{.Task.Definition.ID}}: {{.Task.Definition.Title}}</h1>
<p class="meta">role={{.Task.Definition.Role}} status={{.Task.Status}} owner={{.Task.Owner}} review={{.Task.ReviewStatus}}</p>
{{if .Rollup.Total}}<p class="meta">descendants done: {{.Rollup.Done}}/{{.Rollup.Total}}</p>{{end}}
<form method="post" action="/actions/claim"><input type="hidden" name="csrf" value="{{.CSRFToken}}"><input type="hidden" name="task_id" value="{{.Task.Definition.ID}}"><button type="submit">Claim</button></form>
<h2>Metadata</h2>
<table>
<tr><th>Kind</th><td>{{.Task.Definition.Kind}}</td></tr>
<tr><th>Profile</th><td>{{.Task.Definition.Profile}}</td></tr>
<tr><th>Owning domain</th><td>{{.Task.Definition.OwningDomain}}</td></tr>
<tr><th>Owning layer</th><td>{{.Task.Definition.OwningLayer}}</td></tr>
<tr><th>Source paths</th><td>{{range .Task.Definition.SourcePaths}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Target paths</th><td>{{range .Task.Definition.TargetPaths}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Review domains</th><td>{{range .Task.Definition.ReviewDomains}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Risk</th><td>{{.Task.Definition.RiskLevel}}</td></tr>
<tr><th>Migration type</th><td>{{.Task.Definition.MigrationType}}</td></tr>
</table>
<h2>Notes</h2><pre>{{.Task.Definition.Notes}}</pre>
<h2>Dependencies</h2><ul>{{range .Task.Definition.Dependencies}}<li>{{.}}</li>{{else}}<li>none</li>{{end}}</ul>
<h2>Acceptance</h2><ul>{{range .Task.Definition.AcceptanceChecks}}<li>{{.}}</li>{{else}}<li>none</li>{{end}}</ul>
<h2>History</h2>{{range .Transitions}}<p><code>{{if .FromStatus}}{{.FromStatus}}{{else}}new{{end}} -> {{.ToStatus}}</code> by {{.Actor}} {{.Reason}}</p>{{else}}<p>none</p>{{end}}
<h2>Evidence</h2>{{range .Evidence}}<p><code>{{.Result}}</code> {{.CommandText}} {{.ArtifactPath}}</p>{{else}}<p>none</p>{{end}}
<h2>Handoffs</h2>{{range .Handoffs}}<p>to <b>{{.ToRole}}</b>: {{.Payload}}</p>{{else}}<p>none</p>{{end}}
<h2>Reviews</h2>{{range .Reviews}}<p><b>{{.Verdict}}</b> by {{.Reviewer}}: {{.Reason}}</p>{{else}}<p>none</p>{{end}}
</body></html>`))

func URL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

var multiTemplate = template.Must(template.New("multi").Parse(`<!doctype html>
<html><head><title>fairway multi-project</title><style>
body{font-family:system-ui,sans-serif;margin:32px;background:#f7f7f5;color:#1f2933}
table{border-collapse:collapse;width:100%;background:white;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left}
.project{background:white;border:1px solid #ddd;padding:16px;margin-bottom:24px}.badges{display:flex;gap:8px;flex-wrap:wrap}.badge{border:1px solid #ddd;border-radius:6px;padding:6px 8px}.muted{color:#667085}.status{font-family:monospace}
</style></head><body>
<h1>fairway multi-project</h1>
{{range .Projects}}
<section class="project">
<h2>{{.Name}}</h2>
<p class="muted">{{.Path}}</p>
{{if .Error}}<p>{{.Error}}</p>{{else}}
<div class="badges"><span class="badge">tasks: {{.TaskCount}}</span><span class="badge">sessions: {{.SessionCount}}</span><span class="badge">checkpoints: {{.CheckpointCount}}</span><span class="badge">watchers: {{.WatcherCount}}</span></div>
<table><tr><th>ID</th><th>Title</th><th>Role</th><th>Status</th><th>Review</th></tr>
{{range .Tasks}}<tr><td>{{.Definition.ID}}</td><td>{{.Definition.Title}}</td><td>{{.Definition.Role}}</td><td class="status">{{.Status}}</td><td>{{.ReviewStatus}}</td></tr>{{else}}<tr><td colspan="5">no tasks</td></tr>{{end}}
</table>
{{end}}
</section>
{{else}}<p>no registered projects</p>{{end}}
</body></html>`))
