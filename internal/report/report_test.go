package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yhuan123/spec-driftcheck/internal/runner"
)

// TestRenderJSON_Empty：零 findings 输出 "[]"（不是 "null"，下游 jq 依赖数组语义）。
func TestRenderJSON_Empty(t *testing.T) {
	out, err := RenderJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("want [], got %q", out)
	}
}

// TestRenderJSON_Fields：字段名与 drift-check.yaml 的 reqId 惯例一致，可逆序列化。
func TestRenderJSON_Fields(t *testing.T) {
	out, err := RenderJSON([]runner.Finding{
		{Capability: "D1-demo", ReqID: "REQ-D1-01", Check: "crd-defined", Detail: "未找到 CRD"},
		{Capability: "", ReqID: "", Check: "crd-uncovered", Detail: "NewKind 未登记"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("输出应是合法 JSON 数组: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	first := got[0]
	for k, want := range map[string]string{
		"capability": "D1-demo", "reqId": "REQ-D1-01",
		"check": "crd-defined", "detail": "未找到 CRD",
	} {
		if first[k] != want {
			t.Errorf("first[%q] = %q, want %q", k, first[k], want)
		}
	}
}
