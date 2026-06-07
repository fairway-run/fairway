package dashboard

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type SavedView struct {
	Name     string              `json:"name"`
	Query    string              `json:"query"`
	Filters  map[string][]string `json:"filters,omitempty"`
	Columns  []string            `json:"columns,omitempty"`
	Sort     string              `json:"sort,omitempty"`
	Source   string              `json:"-"`
	Href     string              `json:"-"`
	Shortcut int                 `json:"-"`
}

type savedViewFile struct {
	Views []SavedView `json:"views"`
}

func loadDashboardSavedViews(root string) (personal []SavedView, team []SavedView, err error) {
	personalPath, err := personalSavedViewsPath()
	if err != nil {
		return nil, nil, err
	}
	personal, err = readSavedViews(personalPath, "personal")
	if err != nil {
		return nil, nil, err
	}
	team, err = readSavedViews(teamSavedViewsPath(root), "team")
	if err != nil {
		return nil, nil, err
	}
	for i := range personal {
		if i < 9 {
			personal[i].Shortcut = i + 1
		}
	}
	return personal, team, nil
}

func saveDashboardPersonalView(name, rawQuery string) (SavedView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SavedView{}, errors.New("view name is required")
	}
	view := SavedView{Name: name}
	if err := view.setQuery(rawQuery); err != nil {
		return SavedView{}, err
	}
	path, err := personalSavedViewsPath()
	if err != nil {
		return SavedView{}, err
	}
	views, err := readSavedViews(path, "personal")
	if err != nil {
		return SavedView{}, err
	}
	replaced := false
	for i := range views {
		if views[i].Name == view.Name {
			views[i] = view
			replaced = true
			break
		}
	}
	if !replaced {
		views = append(views, view)
	}
	if err := writeSavedViews(path, views); err != nil {
		return SavedView{}, err
	}
	view.Source = "personal"
	view.Href = savedViewHref(view.Query)
	return view, nil
}

func readSavedViews(path, source string) ([]SavedView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var file savedViewFile
	if err := json.Unmarshal(data, &file); err != nil {
		var list []SavedView
		if listErr := json.Unmarshal(data, &list); listErr != nil {
			return nil, err
		}
		file.Views = list
	}
	var views []SavedView
	for _, view := range file.Views {
		view.Name = strings.TrimSpace(view.Name)
		if view.Name == "" {
			continue
		}
		if err := view.setQuery(view.Query); err != nil {
			return nil, err
		}
		view.Source = source
		view.Href = savedViewHref(view.Query)
		views = append(views, view)
	}
	return views, nil
}

func writeSavedViews(path string, views []SavedView) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file := savedViewFile{Views: views}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (v *SavedView) setQuery(raw string) error {
	query := normalizeSavedViewQuery(raw)
	values, err := url.ParseQuery(query)
	if err != nil {
		return err
	}
	values.Del("page")
	query = values.Encode()
	v.Query = query
	v.Filters = savedViewFilters(values)
	v.Columns = splitSavedViewCSV(values.Get("columns"))
	v.Sort = strings.TrimSpace(values.Get("sort"))
	return nil
}

func savedViewFilters(values url.Values) map[string][]string {
	filters := map[string][]string{}
	for _, key := range []string{"q", "role", "status", "profile", "project", "kind", "owning_domain", "risk_level", "review_domain"} {
		if vals, ok := values[key]; ok {
			var clean []string
			for _, val := range vals {
				val = strings.TrimSpace(val)
				if val != "" {
					clean = append(clean, val)
				}
			}
			if len(clean) > 0 {
				filters[key] = clean
			}
		}
	}
	return filters
}

func normalizeSavedViewQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/board")
	raw = strings.TrimPrefix(raw, "?")
	return raw
}

func savedViewHref(query string) string {
	if strings.TrimSpace(query) == "" {
		return "/board"
	}
	return "/board?" + query
}

func splitSavedViewCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func personalSavedViewsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fairway", "views.json"), nil
}

func teamSavedViewsPath(root string) string {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return filepath.Join(root, ".fairway", "views.json")
}
