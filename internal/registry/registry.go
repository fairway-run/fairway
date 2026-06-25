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
	Name       string `toml:"name" json:"name"`
	Path       string `toml:"path" json:"path"`
	DBPath     string `toml:"db_path,omitempty" json:"db_path,omitempty"`
	ConfigPath string `toml:"config_path,omitempty" json:"config_path,omitempty"`
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
	project.DBPath = normalizeIdentityPath(project.DBPath, project.Path)
	project.ConfigPath = normalizeIdentityPath(project.ConfigPath, project.Path)
	for i, existing := range reg.Projects {
		existing.Path = normalizePath(existing.Path)
		existing.DBPath = normalizeIdentityPath(existing.DBPath, existing.Path)
		existing.ConfigPath = normalizeIdentityPath(existing.ConfigPath, existing.Path)
		if existing.Name == project.Name {
			if !sameProjectIdentity(existing, project) {
				if canUpgradeLegacyProjectIdentity(existing, project) {
					reg.Projects[i] = project
					return reg, nil
				}
				return Registry{}, fmt.Errorf("project name %q is already registered for path=%s db_path=%s config_path=%s", project.Name, existing.Path, existing.DBPath, existing.ConfigPath)
			}
			reg.Projects[i] = project
			return reg, nil
		}
		if sameProjectIdentity(existing, project) {
			return Registry{}, fmt.Errorf("project identity path=%s db_path=%s config_path=%s is already registered as %q", existing.Path, existing.DBPath, existing.ConfigPath, existing.Name)
		}
	}
	reg.Projects = append(reg.Projects, project)
	return reg, nil
}

func ResolveDBPath(project Project) string {
	if project.DBPath == "" {
		return filepath.Join(project.Path, ".fairway", "state.db")
	}
	if filepath.IsAbs(project.DBPath) {
		return filepath.Clean(project.DBPath)
	}
	return filepath.Clean(filepath.Join(project.Path, project.DBPath))
}

func ResolveConfigPath(project Project) string {
	if project.ConfigPath == "" {
		return ""
	}
	if filepath.IsAbs(project.ConfigPath) {
		return filepath.Clean(project.ConfigPath)
	}
	return filepath.Clean(filepath.Join(project.Path, project.ConfigPath))
}

func sameProjectIdentity(left, right Project) bool {
	return filepath.Clean(left.Path) == filepath.Clean(right.Path) &&
		ResolveDBPath(left) == ResolveDBPath(right) &&
		ResolveConfigPath(left) == ResolveConfigPath(right)
}

func canUpgradeLegacyProjectIdentity(existing, project Project) bool {
	if filepath.Clean(existing.Path) != filepath.Clean(project.Path) {
		return false
	}
	if existing.DBPath == "" {
		return true
	}
	return existing.ConfigPath == "" && ResolveDBPath(existing) == ResolveDBPath(project)
}

func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func normalizeIdentityPath(path, root string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
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
