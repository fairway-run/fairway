package dashboard

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/subashram/fairway/internal/store"
)

type Server struct {
	store *store.Store
	roles []string
}

func New(s *store.Store, roles []string) *Server {
	return &Server{store: s, roles: roles}
}

type RoleGroup struct {
	Role    string
	Current *store.Task
	Tasks   []store.Task
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/tasks/", s.task)
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
	data := struct {
		Groups   []RoleGroup
		Activity []store.Activity
		Health   store.Health
	}{groupTasks(tasks, s.roles), activity, health}
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

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/tasks/"):]
	task, transitions, evidence, handoffs, reviews, err := s.store.TaskDetail(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	data := struct {
		Task        store.Task
		Transitions []store.Transition
		Evidence    []store.Evidence
		Handoffs    []store.Handoff
		Reviews     []store.Review
	}{task, transitions, evidence, handoffs, reviews}
	_ = detailTemplate.Execute(w, data)
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
table{border-collapse:collapse;width:100%;background:white;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left}
.status{font-family:monospace}.role{font-weight:600}.badges,.lanes{display:flex;gap:8px;margin:16px 0;flex-wrap:wrap}.badge,.lane{background:#fff;border:1px solid #ddd;padding:6px 8px;border-radius:6px}.lane b{display:block}.layout{display:grid;grid-template-columns:1fr 360px;gap:24px}.feed{background:white;border:1px solid #ddd;padding:12px}.feed p{border-bottom:1px solid #eee;padding-bottom:8px}
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
</div>
<div class="lanes">
{{range .Groups}}<div class="lane"><b>{{.Role}}</b>{{if .Current}}<a href="/tasks/{{.Current.Definition.ID}}">{{.Current.Definition.ID}}</a> {{.Current.Definition.Title}}{{else}}idle{{end}}</div>{{end}}
</div>
<div class="layout">
<main>
{{range .Groups}}
<h2>{{.Role}}</h2>
<table><tr><th>ID</th><th>Title</th><th>Role</th><th>Status</th><th>Owner</th><th>Review</th></tr>
{{range .Tasks}}<tr><td><a href="/tasks/{{.Definition.ID}}">{{.Definition.ID}}</a></td><td>{{.Definition.Title}}</td><td class="role">{{.Definition.Role}}</td><td class="status">{{.Status}}</td><td>{{.Owner}}</td><td>{{.ReviewStatus}}</td></tr>{{else}}<tr><td colspan="6">no tasks</td></tr>{{end}}
</table>
{{end}}
</main>
<aside class="feed"><h2>Activity</h2>{{range .Activity}}<p><b>{{.TaskID}}</b> <code>{{.Kind}}</code><br>{{.Summary}}<br><small>{{.CreatedAt}} {{.Actor}}</small></p>{{else}}<p>none</p>{{end}}</aside>
</div>
</body></html>`))

var detailTemplate = template.Must(template.New("detail").Parse(`<!doctype html>
<html><head><title>{{.Task.Definition.ID}}</title><style>
body{font-family:system-ui,sans-serif;margin:32px;max-width:960px}.meta{color:#555}pre{background:#f4f4f4;padding:12px}
</style></head><body>
<p><a href="/">back</a></p>
<h1>{{.Task.Definition.ID}}: {{.Task.Definition.Title}}</h1>
<p class="meta">role={{.Task.Definition.Role}} status={{.Task.Status}} owner={{.Task.Owner}} review={{.Task.ReviewStatus}}</p>
<h2>Notes</h2><pre>{{.Task.Definition.Notes}}</pre>
<h2>History</h2>{{range .Transitions}}<p><code>{{if .FromStatus}}{{.FromStatus}}{{else}}new{{end}} -> {{.ToStatus}}</code> by {{.Actor}} {{.Reason}}</p>{{else}}<p>none</p>{{end}}
<h2>Evidence</h2>{{range .Evidence}}<p><code>{{.Result}}</code> {{.CommandText}} {{.ArtifactPath}}</p>{{else}}<p>none</p>{{end}}
<h2>Handoffs</h2>{{range .Handoffs}}<p>to <b>{{.ToRole}}</b>: {{.Payload}}</p>{{else}}<p>none</p>{{end}}
<h2>Reviews</h2>{{range .Reviews}}<p><b>{{.Verdict}}</b> by {{.Reviewer}}: {{.Reason}}</p>{{else}}<p>none</p>{{end}}
</body></html>`))

func URL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}
