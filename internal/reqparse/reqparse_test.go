package reqparse

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// 合法文件：一个 P0 双 Scenario 的 REQ + 一个 (P1, planned) 单 Scenario 的 REQ。
const validSpec = `# D2 事件触发

### REQ-D2-01: GitLab push 自动触发流水线 (P0)
**用户故事**：作为开发者…
系统 SHALL 在收到 GitLab push 事件后自动创建并执行 PipelineRun。

#### Scenario: push 后自动创建 PipelineRun
- GIVEN 已配置 GitLab 仓库触发器
- WHEN 开发者向仓库 push 一个 commit
- THEN 系统 SHALL 在 30 秒内自动创建 PipelineRun 并开始执行
- AND PipelineRun 参数 SHALL 包含该 commit 的 SHA 与分支名

#### Scenario: 分支过滤命中时才触发
- GIVEN 触发器仅监听 main 分支
- WHEN 开发者向 feature 分支 push
- THEN 系统 SHALL 不创建 PipelineRun

### REQ-D2-02: Harbor 推送触发扫描 (P1, planned)
**用户故事**：作为安全工程师…
系统 SHALL 在 Harbor 镜像推送后触发扫描流水线。

#### Scenario: 推送后触发扫描
- GIVEN 已配置 Harbor 触发器
- WHEN 推送镜像
- THEN 系统 SHALL 触发扫描流水线
`

func TestParseFile_Valid(t *testing.T) {
	reqs, err := ParseFile(writeTemp(t, validSpec))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("want 2 reqs, got %d: %+v", len(reqs), reqs)
	}

	r0 := reqs[0]
	if r0.ID != "REQ-D2-01" {
		t.Errorf("r0.ID = %q, want REQ-D2-01", r0.ID)
	}
	if r0.Title != "GitLab push 自动触发流水线" {
		t.Errorf("r0.Title = %q", r0.Title)
	}
	if r0.Priority != "P0" {
		t.Errorf("r0.Priority = %q, want P0", r0.Priority)
	}
	if r0.Planned {
		t.Errorf("r0.Planned = true, want false")
	}
	if len(r0.Scenarios) != 2 {
		t.Fatalf("r0 want 2 scenarios, got %d: %+v", len(r0.Scenarios), r0.Scenarios)
	}
	s0 := r0.Scenarios[0]
	if s0.Name != "push 后自动创建 PipelineRun" {
		t.Errorf("s0.Name = %q", s0.Name)
	}
	if !s0.HasGiven || !s0.HasWhen || !s0.HasThen {
		t.Errorf("s0 GWT flags = G:%v W:%v T:%v, want all true", s0.HasGiven, s0.HasWhen, s0.HasThen)
	}
	if reqs[0].Scenarios[1].Name != "分支过滤命中时才触发" {
		t.Errorf("s1.Name = %q", reqs[0].Scenarios[1].Name)
	}

	r1 := reqs[1]
	if r1.ID != "REQ-D2-02" {
		t.Errorf("r1.ID = %q, want REQ-D2-02", r1.ID)
	}
	if r1.Title != "Harbor 推送触发扫描" {
		t.Errorf("r1.Title = %q", r1.Title)
	}
	if r1.Priority != "P1" {
		t.Errorf("r1.Priority = %q, want P1", r1.Priority)
	}
	if !r1.Planned {
		t.Errorf("r1.Planned = false, want true")
	}
	if len(r1.Scenarios) != 1 {
		t.Fatalf("r1 want 1 scenario, got %d", len(r1.Scenarios))
	}
}

func TestParseFile_DuplicateID(t *testing.T) {
	const dup = `### REQ-D1-01: 标题甲 (P0)
#### Scenario: a
- GIVEN x
- WHEN y
- THEN z

### REQ-D1-01: 标题乙 (P1)
#### Scenario: b
- GIVEN x
- WHEN y
- THEN z
`
	if _, err := ParseFile(writeTemp(t, dup)); err == nil {
		t.Fatal("重复 REQ ID 应报错")
	}
}

func TestParseFile_MalformedHeader(t *testing.T) {
	// 以 "### REQ-" 开头但缺括号优先级，应报错。
	const bad = `### REQ-D1-01 标题缺括号优先级
#### Scenario: a
- GIVEN x
- WHEN y
- THEN z
`
	if _, err := ParseFile(writeTemp(t, bad)); err == nil {
		t.Fatal("不匹配完整头格式的 REQ 行应报错")
	}
}

func TestParseFile_ScenarioBeforeAnyReq(t *testing.T) {
	const bad = `# 标题

#### Scenario: 孤儿场景
- GIVEN x
- WHEN y
- THEN z
`
	if _, err := ParseFile(writeTemp(t, bad)); err == nil {
		t.Fatal("首个 REQ 之前出现 Scenario 应报错")
	}
}

// 缺 THEN 的 Scenario：解析不报错，HasThen=false（校验在 runner 做）。
func TestParseFile_MissingThenFlagOnly(t *testing.T) {
	const spec = `### REQ-D1-01: 标题 (P0)
#### Scenario: 缺 THEN
- GIVEN x
- WHEN y
`
	reqs, err := ParseFile(writeTemp(t, spec))
	if err != nil {
		t.Fatalf("缺 THEN 不应报解析错误: %v", err)
	}
	if len(reqs) != 1 || len(reqs[0].Scenarios) != 1 {
		t.Fatalf("结构不符: %+v", reqs)
	}
	s := reqs[0].Scenarios[0]
	if !s.HasGiven || !s.HasWhen {
		t.Errorf("GIVEN/WHEN 应为 true: %+v", s)
	}
	if s.HasThen {
		t.Errorf("HasThen 应为 false: %+v", s)
	}
}
