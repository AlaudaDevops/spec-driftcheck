package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_Valid：一个结构完整、无模糊词、锚点全部命中的能力域 → 零 findings。
func TestRun_Valid(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	repoRoot := filepath.Join(root, "repo")

	mustWrite(t, filepath.Join(repoRoot, "testing/features/ok.feature"),
		"Feature: f\n  Scenario: 存在的场景\n    Given x\n")
	// 供 CRD 校验命中。
	mustWrite(t, filepath.Join(repoRoot, "config/crd/somekind.yaml"), "kind: SomeKind\n")

	capDir := filepath.Join(specDir, "capabilities", "D1-demo")
	mustWrite(t, filepath.Join(capDir, "spec.md"), `# demo

### REQ-D1-01: 合法需求 (P0)
**用户故事**：作为开发者…

#### Scenario: 正常场景
- GIVEN 前置条件
- WHEN 触发操作
- THEN 系统 SHALL 产出结果
`)
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D1\nrepos:\n  tektoncd-operator:\n    paths:\n      - testing/features/**\n    crds: [SomeKind]\n")
	mustWrite(t, filepath.Join(specDir, "sync/repos.yaml"),
		"repos:\n  tektoncd-operator:\n    url: unused\n    branch: main\n    local: true\n")
	mustWrite(t, filepath.Join(specDir, "sync/drift-check.yaml"), "ignore: []\n")

	findings, err := Run(specDir, filepath.Join(root, "work"), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("want 0 findings, got %d: %+v", len(findings), findings)
	}
}

// TestRun_AllFindingClasses：覆盖 5 类 finding。
//   - 无 Scenario 的 REQ        → spec-structure
//   - 缺 THEN 的 Scenario       → spec-structure
//   - 含"合理"的 THEN 行        → fuzzy-word
//   - anchors glob 无匹配        → anchor-path
//   - 不存在的 CRD               → crd-defined
func TestRun_AllFindingClasses(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	repoRoot := filepath.Join(root, "repo")

	mustWrite(t, filepath.Join(repoRoot, "testing/features/ok.feature"),
		"Feature: f\n  Scenario: x\n    Given x\n")

	capDir := filepath.Join(specDir, "capabilities", "D1-demo")
	mustWrite(t, filepath.Join(capDir, "spec.md"), `# demo

### REQ-D1-01: 无场景需求 (P0)
**用户故事**：作为开发者…

### REQ-D1-02: 缺 THEN 需求 (P0)

#### Scenario: 缺结果断言
- GIVEN 前置
- WHEN 操作

### REQ-D1-03: 含模糊词需求 (P1)

#### Scenario: 模糊断言
- GIVEN 前置
- WHEN 操作
- THEN 系统应在合理时间内返回
`)
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D1\nrepos:\n  tektoncd-operator:\n    paths:\n      - testing/features/**\n      - missing/dir/**\n    crds: [NoSuchKind]\n")
	mustWrite(t, filepath.Join(specDir, "sync/repos.yaml"),
		"repos:\n  tektoncd-operator:\n    url: unused\n    branch: main\n    local: true\n")
	mustWrite(t, filepath.Join(specDir, "sync/drift-check.yaml"), "ignore: []\n")

	findings, err := Run(specDir, filepath.Join(root, "work"), repoRoot)
	if err != nil {
		t.Fatal(err)
	}

	byCheck := map[string][]Finding{}
	for _, f := range findings {
		byCheck[f.Check] = append(byCheck[f.Check], f)
	}

	// spec-structure：无 Scenario 的 REQ-D1-01 + 缺 THEN 的 REQ-D1-02 = 2 条。
	ss := byCheck["spec-structure"]
	if len(ss) != 2 {
		t.Errorf("want 2 spec-structure findings, got %d: %+v", len(ss), ss)
	}
	if !anyDetailContains(ss, "REQ-D1-01") {
		t.Errorf("无 Scenario 的 spec-structure finding 应含 REQ-D1-01: %+v", ss)
	}
	if !anyDetailContains(ss, "REQ-D1-02") {
		t.Errorf("缺 THEN 的 spec-structure finding 应含 REQ-D1-02: %+v", ss)
	}
	if !anyDetailContains(ss, "缺结果断言") || !anyDetailContains(ss, "THEN") {
		t.Errorf("缺 THEN finding 应含 Scenario 名与缺失项: %+v", ss)
	}

	// fuzzy-word：含"合理"的 THEN 行 = 1 条。
	fw := byCheck["fuzzy-word"]
	if len(fw) != 1 {
		t.Errorf("want 1 fuzzy-word finding, got %d: %+v", len(fw), fw)
	}
	if len(fw) > 0 && !strings.Contains(fw[0].Detail, "合理") {
		t.Errorf("fuzzy-word finding 应含命中词: %+v", fw[0])
	}

	// anchor-path：missing/dir/** = 1 条。
	if len(byCheck["anchor-path"]) != 1 {
		t.Errorf("want 1 anchor-path finding, got %+v", byCheck["anchor-path"])
	}
	// crd-defined：NoSuchKind = 1 条。
	if len(byCheck["crd-defined"]) != 1 {
		t.Errorf("want 1 crd-defined finding, got %+v", byCheck["crd-defined"])
	}
}

// TestRun_IgnoreByReqID：被 reqId 忽略的 REQ 跳过其 spec-structure / fuzzy-word findings。
func TestRun_IgnoreByReqID(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	repoRoot := filepath.Join(root, "repo")
	mustWrite(t, filepath.Join(repoRoot, "testing/features/ok.feature"),
		"Feature: f\n  Scenario: x\n    Given x\n")

	capDir := filepath.Join(specDir, "capabilities", "D1-demo")
	mustWrite(t, filepath.Join(capDir, "spec.md"), `# demo

### REQ-D1-01: 无场景需求 (P0)
**用户故事**：…
`)
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D1\nrepos:\n  tektoncd-operator:\n    paths:\n      - testing/features/**\n")
	mustWrite(t, filepath.Join(specDir, "sync/repos.yaml"),
		"repos:\n  tektoncd-operator:\n    url: unused\n    branch: main\n    local: true\n")
	mustWrite(t, filepath.Join(specDir, "sync/drift-check.yaml"),
		"ignore:\n  - reqId: REQ-D1-01\n    reason: 重构中\n")

	findings, err := Run(specDir, filepath.Join(root, "work"), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("被忽略的 REQ 应无 finding, got %+v", findings)
	}
}

// TestRun_SpecParseError：spec.md 解析失败 → spec-structure finding，且不中断。
func TestRun_SpecParseError(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	repoRoot := filepath.Join(root, "repo")
	mustWrite(t, filepath.Join(repoRoot, "testing/features/ok.feature"),
		"Feature: f\n  Scenario: x\n    Given x\n")

	capDir := filepath.Join(specDir, "capabilities", "D1-demo")
	// 以 "### REQ-" 开头但格式非法 → 解析错误。
	mustWrite(t, filepath.Join(capDir, "spec.md"), "### REQ-D1-01 缺括号优先级\n")
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D1\nrepos:\n  tektoncd-operator:\n    paths:\n      - testing/features/**\n")
	mustWrite(t, filepath.Join(specDir, "sync/repos.yaml"),
		"repos:\n  tektoncd-operator:\n    url: unused\n    branch: main\n    local: true\n")
	mustWrite(t, filepath.Join(specDir, "sync/drift-check.yaml"), "ignore: []\n")

	findings, err := Run(specDir, filepath.Join(root, "work"), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	var got int
	for _, f := range findings {
		if f.Check == "spec-structure" {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("want 1 spec-structure finding from parse error, got %+v", findings)
	}
}

// TestRun_SkipsNonDirEntries：capabilities/ 下的非目录条目（如 .DS_Store）被跳过，不产生 finding。
func TestRun_SkipsNonDirEntries(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	repoRoot := filepath.Join(root, "repo")
	mustWrite(t, filepath.Join(repoRoot, "testing/features/ok.feature"),
		"Feature: f\n  Scenario: 存在的场景\n    Given x\n")

	capDir := filepath.Join(specDir, "capabilities", "D1-demo")
	mustWrite(t, filepath.Join(capDir, "spec.md"), `# demo

### REQ-D1-01: 合法需求 (P0)
#### Scenario: 正常场景
- GIVEN 前置条件
- WHEN 触发操作
- THEN 系统 SHALL 产出结果
`)
	mustWrite(t, filepath.Join(capDir, "anchors.yaml"),
		"capability: D1\nrepos:\n  tektoncd-operator:\n    paths:\n      - testing/features/**\n")
	mustWrite(t, filepath.Join(specDir, "sync/repos.yaml"),
		"repos:\n  tektoncd-operator:\n    url: unused\n    branch: main\n    local: true\n")
	mustWrite(t, filepath.Join(specDir, "sync/drift-check.yaml"), "ignore: []\n")
	mustWrite(t, filepath.Join(specDir, "capabilities", ".DS_Store"), "junk")

	findings, err := Run(specDir, filepath.Join(root, "work"), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("非目录条目应被跳过, got %+v", findings)
	}
}

func anyDetailContains(fs []Finding, sub string) bool {
	for _, f := range fs {
		if strings.Contains(f.Detail, sub) {
			return true
		}
	}
	return false
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
