// Package notice 实现路径级提醒：PR 变更文件命中 anchors 路径时生成提醒评论。
package notice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yhuan123/spec-driftcheck/internal/anchors"
	"github.com/yhuan123/spec-driftcheck/internal/reqparse"
	"github.com/bmatcuk/doublestar/v4"
)

// Hit 是一次锚点命中。
type Hit struct {
	Capability string
	File       string
	Pattern    string
	ReqIDs     []string // 该能力域 spec.md 的全部 REQ ID（解析失败则留空）
}

// Scan 对照 specDir 下全部 anchors.yaml，返回 changed 中命中 repoName 锚点的文件。
func Scan(specDir, repoName string, changed []string) ([]Hit, error) {
	capDirs, err := filepath.Glob(filepath.Join(specDir, "capabilities", "*"))
	if err != nil {
		return nil, err
	}
	var hits []Hit
	for _, capDir := range capDirs {
		if fi, err := os.Stat(capDir); err != nil || !fi.IsDir() {
			continue // 跳过非目录条目（如 .DS_Store）
		}
		anch, err := anchors.Load(filepath.Join(capDir, "anchors.yaml"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", capDir, err)
		}
		ra, ok := anch.Repos[repoName]
		if !ok {
			continue
		}
		var reqIDs []string
		if reqs, err := reqparse.ParseFile(filepath.Join(capDir, "spec.md")); err == nil {
			for _, req := range reqs {
				reqIDs = append(reqIDs, req.ID)
			}
		}
		for _, f := range changed {
			for _, p := range ra.Paths {
				if ok, _ := doublestar.Match(p, f); ok {
					hits = append(hits, Hit{filepath.Base(capDir), f, p, reqIDs})
					break
				}
			}
		}
	}
	return hits, nil
}

// RenderComment 生成（含幂等 marker 的）PR 评论 markdown。
// specLink 为 spec 目录浏览地址（如 https://github.com/<owner>/<repo>/tree/main/spec/capabilities），空则退化为纯文字。
func RenderComment(hits []Hit, marker, specLink string) string {
	specRef := "spec"
	if specLink != "" {
		specRef = fmt.Sprintf("[spec](%s)", specLink)
	}
	var b strings.Builder
	b.WriteString(marker + "\n## 📋 业务价值 spec 锚点提醒\n\n本 PR 触碰了以下能力域的 spec 锚点路径，若涉及**行为变更**，请同步更新 " + specRef + "：\n\n")
	b.WriteString("| 能力域 | 命中文件 | 关联验收需求 |\n|---|---|---|\n")
	for _, h := range hits {
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", h.Capability, h.File, strings.Join(h.ReqIDs, ", "))
	}
	b.WriteString("\n仅行为不变的重构可忽略本提醒。\n")
	return b.String()
}
