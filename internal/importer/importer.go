package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/subashram/fairway/internal/store"
	"gopkg.in/yaml.v3"
)

type Result struct {
	Tasks  []store.TaskDefinition
	States []store.ImportedTaskState
}

type fileEnvelope struct {
	Tasks []taskRecord `json:"tasks" yaml:"tasks"`
}

type taskRecord struct {
	store.TaskDefinition `yaml:",inline"`
	DependsOn            []string `json:"depends_on" yaml:"depends_on"`
	Status               string   `json:"status" yaml:"status"`
	Owner                string   `json:"owner" yaml:"owner"`
	Branch               string   `json:"branch" yaml:"branch"`
	CompletedAt          string   `json:"completed_at" yaml:"completed_at"`
	Commit               string   `json:"commit" yaml:"commit"`
	CommitSHA            string   `json:"commit_sha" yaml:"commit_sha"`
}

func Tasks(path string) ([]store.TaskDefinition, error) {
	result, err := File(path)
	if err != nil {
		return nil, err
	}
	return result.Tasks, nil
}

func File(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	var records []taskRecord
	if strings.HasSuffix(path, ".json") {
		if err := json.Unmarshal(data, &records); err == nil {
			return normalize(records), nil
		}
		var envelope fileEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return Result{}, fmt.Errorf("parse json: %w", err)
		}
		return normalize(envelope.Tasks), nil
	}
	if err := yaml.Unmarshal(data, &records); err == nil && len(records) > 0 {
		return normalize(records), nil
	}
	var envelope fileEnvelope
	if err := yaml.Unmarshal(data, &envelope); err != nil {
		return Result{}, fmt.Errorf("parse yaml: %w", err)
	}
	return normalize(envelope.Tasks), nil
}

func normalize(records []taskRecord) Result {
	result := Result{
		Tasks:  make([]store.TaskDefinition, 0, len(records)),
		States: make([]store.ImportedTaskState, 0, len(records)),
	}
	for _, record := range records {
		task := record.TaskDefinition
		if len(task.Dependencies) == 0 && len(record.DependsOn) > 0 {
			task.Dependencies = record.DependsOn
		}
		result.Tasks = append(result.Tasks, task)
		commit := record.CommitSHA
		if commit == "" {
			commit = record.Commit
		}
		if record.Status != "" || record.Owner != "" || record.Branch != "" || record.CompletedAt != "" || commit != "" {
			result.States = append(result.States, store.ImportedTaskState{
				TaskID:      task.ID,
				Status:      record.Status,
				Owner:       record.Owner,
				Branch:      record.Branch,
				CompletedAt: record.CompletedAt,
				CommitSHA:   commit,
			})
		}
	}
	return result
}
