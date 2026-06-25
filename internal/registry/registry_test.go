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

func TestRegisterAllowsSamePathWithDifferentDBAndName(t *testing.T) {
	root := t.TempDir()
	reg, err := Register(Registry{}, Project{Name: "platform", Path: root, DBPath: ".fairway/platform.db", ConfigPath: ".fairway/platform.toml"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = Register(reg, Project{Name: "docs", Path: root, DBPath: ".fairway/docs.db", ConfigPath: ".fairway/docs.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 2 {
		t.Fatalf("registry=%+v, want two same-path projects", reg)
	}
	if reg.Projects[0].Path != reg.Projects[1].Path {
		t.Fatalf("paths differ unexpectedly: %+v", reg.Projects)
	}
	if reg.Projects[0].DBPath == reg.Projects[1].DBPath {
		t.Fatalf("db identity not preserved: %+v", reg.Projects)
	}
}

func TestRegisterRejectsDuplicateIdentityWithDifferentName(t *testing.T) {
	root := t.TempDir()
	reg, err := Register(Registry{}, Project{Name: "platform", Path: root, DBPath: ".fairway/state.db"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Register(reg, Project{Name: "platform-copy", Path: root, DBPath: ".fairway/state.db"}); err == nil {
		t.Fatal("expected duplicate same-path same-db identity to fail")
	}
}

func TestRegisterUpgradesLegacyPathOnlyProjectIdentity(t *testing.T) {
	root := t.TempDir()
	reg := Registry{Projects: []Project{{Name: "platform", Path: root}}}
	reg, err := Register(reg, Project{Name: "platform", Path: root, DBPath: ".fairway/platform.db", ConfigPath: ".fairway/platform.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 1 {
		t.Fatalf("registry=%+v, want one upgraded project", reg)
	}
	if reg.Projects[0].DBPath == "" || reg.Projects[0].ConfigPath == "" {
		t.Fatalf("registry=%+v, want DB/config identity filled", reg)
	}
}

func TestRegisterUpgradesLegacyDBOnlyProjectIdentity(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, ".fairway", "platform.db")
	reg := Registry{Projects: []Project{{Name: "platform", Path: root, DBPath: dbPath}}}
	reg, err := Register(reg, Project{Name: "platform", Path: root, DBPath: dbPath, ConfigPath: ".fairway/platform.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 1 || reg.Projects[0].ConfigPath == "" {
		t.Fatalf("registry=%+v, want config identity filled", reg)
	}
}

func TestRegisterRejectsSameNameSamePathDifferentDBWhenIdentityKnown(t *testing.T) {
	root := t.TempDir()
	reg, err := Register(Registry{}, Project{Name: "platform", Path: root, DBPath: ".fairway/platform.db", ConfigPath: ".fairway/platform.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Register(reg, Project{Name: "platform", Path: root, DBPath: ".fairway/other.db", ConfigPath: ".fairway/platform.toml"}); err == nil {
		t.Fatal("expected same name/path with conflicting known DB identity to fail")
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.toml")
	want := Registry{Projects: []Project{{Name: "fairway", Path: "/tmp/fairway", ConfigPath: "/tmp/fairway/.fairway/config.toml"}}}
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
	if got.Projects[0].ConfigPath == "" {
		t.Fatalf("registry=%+v, want config path round trip", got)
	}
}
