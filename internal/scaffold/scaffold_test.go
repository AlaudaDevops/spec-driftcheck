package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhuan123/spec-driftcheck/internal/runner"
)

var testParams = Params{
	PluginName: "demo-plugin",
	SpecRepo:   "acme/demo-plugin",
	ToolImage:  "ghcr.io/yhuan123/spec-driftcheck:latest",
}

// TestRender_InitIsGreen 断言：脚手架生成后直接通过 driftcheck check（零 findings）。
func TestRender_InitIsGreen(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	written, err := Render(specDir, testParams)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) == 0 {
		t.Fatal("应至少渲染一个文件")
	}
	findings, err := runner.Run(specDir, filepath.Join(root, "work"), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("脚手架应零 findings, got %+v", findings)
	}
}

// TestRender_SubstitutesParams 断言：模板变量被替换且无残留占位符。
func TestRender_SubstitutesParams(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	written, err := Render(specDir, testParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range written {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "{{") {
			t.Errorf("%s 含未渲染的模板占位符", f)
		}
	}
	task, err := os.ReadFile(filepath.Join(specDir, "sync/tasks/spec-drift-notice.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(task), "https://github.com/acme/demo-plugin.git") {
		t.Errorf("Task 应包含渲染后的 spec-repo-url: %s", task)
	}
}

// TestRender_RefusesOverwrite 断言：目标文件已存在时报错，不覆盖。
func TestRender_RefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	if _, err := Render(specDir, testParams); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(specDir, testParams); err == nil {
		t.Fatal("重复渲染应报错拒绝覆盖")
	}
}
