package registry

import (
	"path/filepath"
	"testing"
)

func TestRegisterIdempotentByNameAndPath(t *testing.T) {
	reg, err := Register(Registry{}, Project{Name: "fairway", Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = Register(reg, Project{Name: "fairway", Path: reg.Projects[0].Path, DBPath: ".fairway/state.db"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 1 || reg.Projects[0].DBPath == "" {
		t.Fatalf("registry=%+v, want one updated project", reg)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.toml")
	want := Registry{Projects: []Project{{Name: "fairway", Path: "/tmp/fairway"}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "fairway" {
		t.Fatalf("registry=%+v, want fairway", got)
	}
}
