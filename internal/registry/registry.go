package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Registry struct {
	Projects []Project `toml:"project" json:"projects"`
}

type Project struct {
	Name   string `toml:"name" json:"name"`
	Path   string `toml:"path" json:"path"`
	DBPath string `toml:"db_path,omitempty" json:"db_path,omitempty"`
}

func DefaultPath() (string, error) {
	if path := os.Getenv("FAIRWAY_REGISTRY"); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fairway", "registry.toml"), nil
}

func Load(path string) (Registry, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Registry{}, err
		}
	}
	var reg Registry
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return reg, nil
	}
	if _, err := toml.DecodeFile(path, &reg); err != nil {
		return Registry{}, err
	}
	return reg, nil
}

func Save(path string, reg Registry) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(reg)
}

func Register(reg Registry, project Project) (Registry, error) {
	if project.Name == "" {
		return Registry{}, errors.New("project name is required")
	}
	if project.Path == "" {
		return Registry{}, errors.New("project path is required")
	}
	abs, err := filepath.Abs(project.Path)
	if err != nil {
		return Registry{}, err
	}
	project.Path = abs
	for i, existing := range reg.Projects {
		if existing.Name == project.Name {
			if filepath.Clean(existing.Path) != filepath.Clean(project.Path) {
				return Registry{}, fmt.Errorf("project name %q is already registered for %s", project.Name, existing.Path)
			}
			reg.Projects[i] = project
			return reg, nil
		}
		if filepath.Clean(existing.Path) == filepath.Clean(project.Path) {
			reg.Projects[i] = project
			return reg, nil
		}
	}
	reg.Projects = append(reg.Projects, project)
	return reg, nil
}

func Unregister(reg Registry, name string) (Registry, bool) {
	out := reg.Projects[:0]
	removed := false
	for _, project := range reg.Projects {
		if project.Name == name {
			removed = true
			continue
		}
		out = append(out, project)
	}
	reg.Projects = out
	return reg, removed
}
