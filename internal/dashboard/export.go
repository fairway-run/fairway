package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

type boardExportPayload struct {
	Format       string              `json:"format"`
	FilteredRows int                 `json:"filtered_rows"`
	ExportedRows int                 `json:"exported_rows"`
	Columns      []string            `json:"columns"`
	Rows         []map[string]string `json:"rows"`
}

func (s *Server) boardExport(w http.ResponseWriter, r *http.Request) {
	payload, err := s.boardExportPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch strings.TrimSpace(r.URL.Query().Get("format")) {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	default:
		writeBoardExportCSV(w, payload)
	}
}

func (s *Server) boardExportPayload(r *http.Request) (boardExportPayload, error) {
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		return boardExportPayload{}, err
	}
	tasks = tagTasksProject(tasks, s.cfg.Fairway.ProjectName)
	return boardExportPayloadFromTasks(r, tasks, s.cfg.Fairway.ProjectName), nil
}

func (s *MultiServer) boardExport(w http.ResponseWriter, r *http.Request) {
	tasks, _, _, _, _, err := s.projectFacts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := boardExportPayloadFromTasks(r, tasks, "")
	switch strings.TrimSpace(r.URL.Query().Get("format")) {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	default:
		writeBoardExportCSV(w, payload)
	}
}

func boardExportPayloadFromTasks(r *http.Request, tasks []store.Task, projectName string) boardExportPayload {
	filters := taskFiltersFromRequest(r)
	filtered := filterTasks(tasks, filters, projectName)
	sortBoardRows(filtered, filters)
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	columns := boardVisibleColumns(boardColumns(filters))
	payload := boardExportPayload{
		Format:       strings.TrimSpace(r.URL.Query().Get("format")),
		FilteredRows: len(filtered),
		ExportedRows: len(filtered),
		Columns:      boardExportColumnLabels(columns),
		Rows:         boardExportRows(filtered, columns, rollups),
	}
	if payload.Format == "" {
		payload.Format = "csv"
	}
	return payload
}

func boardExportColumnLabels(columns []BoardColumn) []string {
	labels := make([]string, 0, len(columns))
	for _, column := range columns {
		labels = append(labels, column.Label)
	}
	return labels
}

func boardExportRows(tasks []store.Task, columns []BoardColumn, rollups map[string]Rollup) []map[string]string {
	rows := make([]map[string]string, 0, len(tasks))
	for _, task := range tasks {
		row := map[string]string{}
		for _, column := range columns {
			row[column.Label] = boardTaskPlainCell(task, column, rollups)
		}
		rows = append(rows, row)
	}
	return rows
}

func writeBoardExportCSV(w http.ResponseWriter, payload boardExportPayload) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fairway-board.csv"`)
	w.Header().Set("X-Fairway-Filtered-Rows", strconv.Itoa(payload.FilteredRows))
	w.Header().Set("X-Fairway-Exported-Rows", strconv.Itoa(payload.ExportedRows))
	writer := csv.NewWriter(w)
	_ = writer.Write(payload.Columns)
	for _, row := range payload.Rows {
		values := make([]string, 0, len(payload.Columns))
		for _, column := range payload.Columns {
			values = append(values, row[column])
		}
		_ = writer.Write(values)
	}
	writer.Flush()
}
