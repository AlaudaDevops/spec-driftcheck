// Package report 渲染漂移检测结果。
package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yhuan123/spec-driftcheck/internal/runner"
)

// Render 输出 markdown 报告。
func Render(findings []runner.Finding) string {
	if len(findings) == 0 {
		return "✅ spec 漂移检测通过：所有锚点有效。\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## ⚠️ spec 漂移检测发现 %d 处问题\n\n", len(findings))
	b.WriteString("| 能力域 | REQ | 校验项 | 详情 |\n|---|---|---|---|\n")
	for _, f := range findings {
		id := f.ReqID
		if id == "" {
			id = "-"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", f.Capability, id, f.Check, f.Detail)
	}
	b.WriteString("\n处理方式：更新 spec/对应 REQ，或在 spec/sync/drift-check.yaml 中附原因临时豁免。\n")
	return b.String()
}

// RenderJSON 输出 findings 的 JSON 数组（零 findings 输出 []，供下游 jq 消费）。
func RenderJSON(findings []runner.Finding) (string, error) {
	if len(findings) == 0 {
		return "[]", nil
	}
	data, err := json.MarshalIndent(findings, "", "  ")
	return string(data), err
}
