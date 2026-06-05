package anchors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "repo", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "repo", "a", "b.feature"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(dir, "anchors.yaml")
	content := "capability: D9\nrepos:\n  myrepo:\n    paths:\n      - a/**\n      - missing/**\n    crds: [Foo]\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if f.Capability != "D9" || len(f.Repos["myrepo"].Paths) != 2 {
		t.Fatalf("unexpected parse: %+v", f)
	}

	bad := ValidatePaths(filepath.Join(dir, "repo"), f.Repos["myrepo"].Paths)
	if len(bad) != 1 || bad[0] != "missing/**" {
		t.Errorf("应只有 missing/** 失效, got %v", bad)
	}
}

func TestCRDDefined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.yaml"), []byte("kind: ScheduledTrigger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !CRDDefined(dir, "ScheduledTrigger") {
		t.Error("应能在 yaml 中找到 kind: ScheduledTrigger")
	}
	if CRDDefined(dir, "NotExist") {
		t.Error("不存在的 CRD 不应命中")
	}
}
