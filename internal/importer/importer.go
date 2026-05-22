package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/subashram/fairway/internal/store"
	"gopkg.in/yaml.v3"
)

func Tasks(path string) ([]store.TaskDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tasks []store.TaskDefinition
	if strings.HasSuffix(path, ".json") {
		if err := json.Unmarshal(data, &tasks); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		return tasks, nil
	}
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return tasks, nil
}
