// Package runner 编排 check 子命令的全部校验。
package runner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yhuan123/spec-driftcheck/internal/anchors"
	"github.com/yhuan123/spec-driftcheck/internal/reqparse"
	"github.com/yhuan123/spec-driftcheck/internal/repos"
	"gopkg.in/yaml.v3"
)

// Finding 是一处漂移。
type Finding struct {
	Capability string `json:"capability"`
	ReqID      string `json:"reqId"` // anchors/CRD 级问题为空；fuzzy-word 可留空（见 Detail）
	Check      string `json:"check"` // spec-structure | fuzzy-word | anchor-path | crd-defined | crd-uncovered
	Detail     string `json:"detail"`
}

type ignoreFile struct {
	Ignore []struct {
		ReqID  string `yaml:"reqId"`
		Reason string `yaml:"reason"`
	} `yaml:"ignore"`
	// IgnoreCRDs 豁免反向 CRD 哨兵（如上游内置/测试 fixture 的 kind）。
	IgnoreCRDs []string `yaml:"ignoreCRDs"`
}

// 模糊词清单：spec GWT 行不应出现这些主观/含糊措辞。
var fuzzyWords = []string{"合理", "正常", "适当", "尽快", "友好"}

// GWT 行前缀：- GIVEN/WHEN/THEN/AND。
var gwtLineRe = regexp.MustCompile(`^-\s+(GIVEN|WHEN|THEN|AND)\b`)

// Run 执行全部校验。基础设施错误（克隆失败等）返回 error；漂移返回 findings。
func Run(specDir, workDir, localRepoRoot string) ([]Finding, error) {
	reposFile, err := repos.Load(filepath.Join(specDir, "sync", "repos.yaml"))
	if err != nil {
		return nil, err
	}
	ignored, ignoredCRDs, err := loadIgnore(filepath.Join(specDir, "sync", "drift-check.yaml"))
	if err != nil {
		return nil, err
	}

	capDirs, err := filepath.Glob(filepath.Join(specDir, "capabilities", "*"))
	if err != nil {
		return nil, err
	}

	repoDirCache := map[string]string{}
	resolve := func(name string) (string, error) {
		if dir, ok := repoDirCache[name]; ok {
			return dir, nil
		}
		r, ok := reposFile.Repos[name]
		if !ok {
			return "", fmt.Errorf("仓库 %q 未在 sync/repos.yaml 登记", name)
		}
		dir, err := repos.Dir(name, r, localRepoRoot, workDir)
		if err != nil {
			return "", err
		}
		repoDirCache[name] = dir
		return dir, nil
	}

	var findings []Finding
	registeredCRDs := map[string]bool{} // 全部 anchors 登记的 CRD 并集（反向哨兵基准）
	for _, capDir := range capDirs {
		if fi, err := os.Stat(capDir); err != nil || !fi.IsDir() {
			continue // 跳过非目录条目（如 .DS_Store）
		}
		capName := filepath.Base(capDir)
		specPath := filepath.Join(capDir, "spec.md")

		reqs, err := reqparse.ParseFile(specPath)
		if err != nil {
			findings = append(findings, Finding{Capability: capName, Check: "spec-structure", Detail: err.Error()})
			continue
		}

		// 结构校验：每个 REQ 必须有 Scenario，每个 Scenario 必须有 GIVEN/WHEN/THEN。
		for _, req := range reqs {
			if ignored[req.ID] {
				continue
			}
			if len(req.Scenarios) == 0 {
				findings = append(findings, Finding{capName, req.ID, "spec-structure",
					fmt.Sprintf("%s 没有任何 Scenario", req.ID)})
				continue
			}
			for _, s := range req.Scenarios {
				var missing []string
				if !s.HasGiven {
					missing = append(missing, "GIVEN")
				}
				if !s.HasWhen {
					missing = append(missing, "WHEN")
				}
				if !s.HasThen {
					missing = append(missing, "THEN")
				}
				if len(missing) > 0 {
					findings = append(findings, Finding{capName, req.ID, "spec-structure",
						fmt.Sprintf("%s Scenario %q 缺少 %s", req.ID, s.Name, strings.Join(missing, "/"))})
				}
			}
		}

		// 模糊词 lint：扫描 GWT 行；归属当前 REQ（被忽略的 REQ 跳过）。
		findings = append(findings, fuzzyLint(specPath, capName, ignored)...)

		anch, err := anchors.Load(filepath.Join(capDir, "anchors.yaml"))
		if err != nil {
			findings = append(findings, Finding{Capability: capName, Check: "spec-structure", Detail: "anchors.yaml: " + err.Error()})
			continue
		}
		for repoName, ra := range anch.Repos {
			repoDir, err := resolve(repoName)
			if err != nil {
				return nil, err
			}
			for _, bad := range anchors.ValidatePaths(repoDir, ra.Paths) {
				findings = append(findings, Finding{capName, "", "anchor-path",
					fmt.Sprintf("%s: glob %q 无匹配文件", repoName, bad)})
			}
			for _, crd := range ra.CRDs {
				registeredCRDs[crd] = true
				if !anchors.CRDDefined(repoDir, crd) {
					findings = append(findings, Finding{capName, "", "crd-defined",
						fmt.Sprintf("%s: 未找到 CRD %q 定义", repoName, crd)})
				}
			}
		}
	}

	// 反向 CRD 哨兵：仓库中已定义但未被任何 anchors 登记的 CRD —— 可能是 spec 未覆盖的新能力面。
	for repoName, repoDir := range repoDirCache {
		for _, kind := range anchors.DiscoverCRDs(repoDir) {
			if !registeredCRDs[kind] && !ignoredCRDs[kind] {
				findings = append(findings, Finding{"", "", "crd-uncovered",
					fmt.Sprintf("%s: CRD %q 已定义但未被任何 anchors 登记（新能力面？补 anchors+REQ 或加入 ignoreCRDs）", repoName, kind)})
			}
		}
	}
	return findings, nil
}

// fuzzyLint 重新读取 spec.md，对每条 GWT 行检查模糊词。
// 命中行归属其上方最近的 REQ；被忽略的 REQ 跳过。
func fuzzyLint(specPath, capName string, ignored map[string]bool) []Finding {
	f, err := os.Open(specPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []Finding
	curReq := ""
	curIgnored := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := reqparse.MatchHeader(line); m != "" {
			curReq = m
			curIgnored = ignored[m]
			continue
		}
		if !gwtLineRe.MatchString(line) || curIgnored {
			continue
		}
		for _, w := range fuzzyWords {
			if strings.Contains(line, w) {
				out = append(out, Finding{capName, curReq, "fuzzy-word",
					fmt.Sprintf("模糊词 %q 出现在 %s 的 GWT 行: %s", w, reqOrDash(curReq), truncate(line, 60))})
				break // 每行至多报一次
			}
		}
	}
	return out
}

func reqOrDash(id string) string {
	if id == "" {
		return "(REQ 之前)"
	}
	return id
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func loadIgnore(path string) (reqIDs, crds map[string]bool, err error) {
	reqIDs, crds = map[string]bool{}, map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reqIDs, crds, nil
		}
		return nil, nil, err
	}
	var f ignoreFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, nil, err
	}
	for _, i := range f.Ignore {
		reqIDs[i.ReqID] = true
	}
	for _, c := range f.IgnoreCRDs {
		crds[c] = true
	}
	return reqIDs, crds, nil
}
