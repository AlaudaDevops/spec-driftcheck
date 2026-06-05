package notice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const specWithTwoReqs = `# D2 事件触发

### REQ-D2-01: GitLab push 自动触发流水线 (P0)
#### Scenario: a
- GIVEN x
- WHEN y
- THEN z

### REQ-D2-02: Harbor 推送触发扫描 (P1, planned)
#### Scenario: b
- GIVEN x
- WHEN y
- THEN z
`

func TestScan(t *testing.T) {
	specDir := t.TempDir()
	capDir := filepath.Join(specDir, "capabilities", "D2-demo")
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D2\nrepos:\n  tektoncd-triggers:\n    paths:\n      - test/integration/features/**\n")
	mustWrite(t, filepath.Join(capDir, "spec.md"), specWithTwoReqs)

	hits, err := Scan(specDir, "tektoncd-triggers",
		[]string{"test/integration/features/gitlab-bindings.feature", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %+v", hits)
	}
	h := hits[0]
	if h.Capability != "D2-demo" || h.File != "test/integration/features/gitlab-bindings.feature" {
		t.Errorf("unexpected hit: %+v", h)
	}
	// ReqIDs 为该能力域 spec.md 的全部 REQ ID（无 repo 过滤）。
	if len(h.ReqIDs) != 2 || h.ReqIDs[0] != "REQ-D2-01" || h.ReqIDs[1] != "REQ-D2-02" {
		t.Errorf("want [REQ-D2-01 REQ-D2-02], got %v", h.ReqIDs)
	}
}

// TestScan_SkipsMissingAnchors 断言：缺失 anchors.yaml 的能力域目录被跳过而非硬失败。
func TestScan_SkipsMissingAnchors(t *testing.T) {
	specDir := t.TempDir()
	capDir := filepath.Join(specDir, "capabilities", "D2-demo")
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D2\nrepos:\n  tektoncd-triggers:\n    paths:\n      - test/integration/features/**\n")
	mustWrite(t, filepath.Join(capDir, "spec.md"), specWithTwoReqs)
	noAnchorDir := filepath.Join(specDir, "capabilities", "D9-noanchor")
	mustWrite(t, filepath.Join(noAnchorDir, "spec.md"), "no anchors here\n")

	hits, err := Scan(specDir, "tektoncd-triggers",
		[]string{"test/integration/features/gitlab-bindings.feature", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %+v", hits)
	}
	if hits[0].Capability != "D2-demo" {
		t.Errorf("unexpected hit: %+v", hits[0])
	}
}

// TestScan_SpecParseFailureLeavesReqIDsEmpty 断言：spec.md 解析失败时 ReqIDs 留空，仍命中。
func TestScan_SpecParseFailureLeavesReqIDsEmpty(t *testing.T) {
	specDir := t.TempDir()
	capDir := filepath.Join(specDir, "capabilities", "D2-demo")
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D2\nrepos:\n  tektoncd-triggers:\n    paths:\n      - test/integration/features/**\n")
	// 非法 REQ 头 → 解析失败。
	mustWrite(t, filepath.Join(capDir, "spec.md"), "### REQ-D2-01 缺括号优先级\n")

	hits, err := Scan(specDir, "tektoncd-triggers",
		[]string{"test/integration/features/x.feature"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %+v", hits)
	}
	if len(hits[0].ReqIDs) != 0 {
		t.Errorf("解析失败时 ReqIDs 应留空, got %v", hits[0].ReqIDs)
	}
}

// TestScan_SkipsNonDirEntries 断言：capabilities/ 下的非目录条目（如 .DS_Store）被跳过而非硬失败。
func TestScan_SkipsNonDirEntries(t *testing.T) {
	specDir := t.TempDir()
	capDir := filepath.Join(specDir, "capabilities", "D2-demo")
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D2\nrepos:\n  tektoncd-triggers:\n    paths:\n      - test/integration/features/**\n")
	mustWrite(t, filepath.Join(capDir, "spec.md"), specWithTwoReqs)
	mustWrite(t, filepath.Join(specDir, "capabilities", ".DS_Store"), "junk")

	hits, err := Scan(specDir, "tektoncd-triggers",
		[]string{"test/integration/features/gitlab-bindings.feature"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Capability != "D2-demo" {
		t.Fatalf("want 1 hit from D2-demo, got %+v", hits)
	}
}

func TestRenderComment_HeaderAndReqIDs(t *testing.T) {
	hits := []Hit{{
		Capability: "D2-demo",
		File:       "test/x.feature",
		Pattern:    "test/**",
		ReqIDs:     []string{"REQ-D2-01", "REQ-D2-02"},
	}}
	out := RenderComment(hits, "<!-- m -->", "https://example.com/spec/capabilities")
	if !strings.Contains(out, "关联验收需求") {
		t.Errorf("表头应为 关联验收需求: %s", out)
	}
	if strings.Contains(out, "关联验收标准") {
		t.Errorf("不应再出现旧表头 关联验收标准")
	}
	if !strings.Contains(out, "REQ-D2-01, REQ-D2-02") {
		t.Errorf("应渲染 REQ ID 列表: %s", out)
	}
	if !strings.Contains(out, "[spec](https://example.com/spec/capabilities)") {
		t.Errorf("应渲染 spec 链接: %s", out)
	}
}

// TestRenderComment_NoSpecLink 断言：specLink 为空时退化为纯文字，不渲染 markdown 链接。
func TestRenderComment_NoSpecLink(t *testing.T) {
	hits := []Hit{{Capability: "D2-demo", File: "x", Pattern: "x", ReqIDs: []string{"REQ-D2-01"}}}
	out := RenderComment(hits, "<!-- m -->", "")
	if strings.Contains(out, "[spec](") {
		t.Errorf("specLink 为空时不应渲染链接: %s", out)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
