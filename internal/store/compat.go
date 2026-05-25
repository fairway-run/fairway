package store

import (
	"fmt"
	"sort"
	"strings"
)

type CompatFinding struct {
	Migration string `json:"migration"`
	Token     string `json:"token"`
	Message   string `json:"message"`
}

type CompatReport struct {
	Backend  string          `json:"backend"`
	OK       bool            `json:"ok"`
	Files    []string        `json:"files"`
	Findings []CompatFinding `json:"findings"`
}

func PostgresCompatReport() (CompatReport, error) {
	report := CompatReport{Backend: "postgres", OK: true}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return CompatReport{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()
		report.Files = append(report.Files, name)
		data, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return CompatReport{}, err
		}
		sql := strings.ToUpper(string(data))
		for token, msg := range postgresForbiddenTokens() {
			if strings.Contains(sql, token) {
				report.Findings = append(report.Findings, CompatFinding{Migration: name, Token: token, Message: msg})
			}
		}
	}
	report.OK = len(report.Findings) == 0
	return report, nil
}

func PostgresCompatDDL() (string, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out strings.Builder
	out.WriteString("-- fairway postgres compatibility DDL generated from embedded migrations\n")
	out.WriteString("-- Review before applying; runtime support remains SQLite-first.\n\n")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return "", err
		}
		out.WriteString(fmt.Sprintf("-- %s\n", entry.Name()))
		out.Write(data)
		out.WriteString("\n\n")
	}
	return out.String(), nil
}

func postgresForbiddenTokens() map[string]string {
	return map[string]string{
		"AUTOINCREMENT": "SQLite autoincrement syntax does not translate to Postgres.",
		"INSERT OR ":    "SQLite conflict syntax must be rewritten as ON CONFLICT.",
		"PRAGMA ":       "SQLite pragmas must stay inside sqlite adapters.",
		"VACUUM ":       "SQLite maintenance commands do not belong in portable migrations.",
		"JSON_EXTRACT":  "SQLite JSON1 functions do not translate directly to Postgres.",
		"WITHOUT ROWID": "SQLite table storage directives do not translate to Postgres.",
	}
}
