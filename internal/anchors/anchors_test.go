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

// TestDiscoverCRDs 断言：从 CustomResourceDefinition manifest 中提取 spec.names.kind，
// 支持多 doc yaml；普通 manifest 的 kind 不算；vendor 跳过。
func TestDiscoverCRDs(t *testing.T) {
	dir := t.TempDir()
	multiDoc := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.example.com
spec:
  names:
    kind: Foo
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-crd
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: bars.example.com
spec:
  names:
    kind: Bar
`
	if err := os.WriteFile(filepath.Join(dir, "crds.yaml"), []byte(multiDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	// 普通资源引用 kind: Baz —— 不是 CRD 定义，不应被发现。
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("kind: Baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// vendor 下的 CRD 应跳过。
	if err := os.MkdirAll(filepath.Join(dir, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	vendorCRD := "kind: CustomResourceDefinition\nspec:\n  names:\n    kind: Vendored\n"
	if err := os.WriteFile(filepath.Join(dir, "vendor", "v.yaml"), []byte(vendorCRD), 0o644); err != nil {
		t.Fatal(err)
	}

	kinds := DiscoverCRDs(dir)
	want := map[string]bool{"Foo": true, "Bar": true}
	if len(kinds) != 2 || !want[kinds[0]] || !want[kinds[1]] {
		t.Fatalf("want [Foo Bar], got %v", kinds)
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
