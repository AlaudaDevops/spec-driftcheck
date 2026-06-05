// Package reqparse 解析 spec.md 中的独立业务用例（REQ + Scenario，GWT 格式）。
package reqparse

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

// Scenario 是一个 REQ 下的场景及其 GWT 标志位。
type Scenario struct {
	Name                       string
	HasGiven, HasWhen, HasThen bool
}

// Req 是一条独立业务需求。
type Req struct {
	ID        string
	Title     string
	Priority  string // P0|P1|P2
	Planned   bool
	Scenarios []Scenario
}

var (
	// 完整 REQ 头：### REQ-<大写字母+可选数字>-<数字>: <标题> (P0|P1|P2[, planned])
	reqHeadRe = regexp.MustCompile(`^###\s+(REQ-[A-Z]+\d*-\d+):\s+(.+?)\s+\((P[0-2])(,\s*planned)?\)\s*$`)
	// 触发判断：以 "### REQ-" 开头的行都应当是合法 REQ 头，否则报格式错误。
	reqTriggerRe = regexp.MustCompile(`^###\s+REQ-`)
	scenarioRe   = regexp.MustCompile(`^####\s+Scenario:\s*(.+?)\s*$`)
	gwtRe        = regexp.MustCompile(`^-\s+(GIVEN|WHEN|THEN|AND)\b`)
)

// MatchHeader 若 line 是合法 REQ 头则返回其 REQ ID，否则返回空串。
// 供 lint 复用同一份头格式契约，避免正则漂移。
func MatchHeader(line string) string {
	if m := reqHeadRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// ParseFile 解析 spec.md，返回全部 REQ。
//
// 错误情形：
//   - REQ ID 在文件内重复；
//   - 以 "### REQ-" 开头但不匹配完整头格式的行；
//   - "#### Scenario:" 出现在任何 REQ 之前。
func ParseFile(path string) ([]Req, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reqs []Req
	seen := map[string]bool{}
	cur := -1 // 当前 REQ 在 reqs 中的下标
	curScenario := -1

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()

		if m := reqHeadRe.FindStringSubmatch(line); m != nil {
			id := m[1]
			if seen[id] {
				return nil, fmt.Errorf("%s:%d: REQ ID 重复: %s", path, n, id)
			}
			seen[id] = true
			reqs = append(reqs, Req{ID: id, Title: m[2], Priority: m[3], Planned: m[4] != ""})
			cur = len(reqs) - 1
			curScenario = -1
			continue
		}
		// 以 "### REQ-" 开头却未匹配完整头格式 → 格式错误。
		if reqTriggerRe.MatchString(line) {
			return nil, fmt.Errorf("%s:%d: REQ 头格式非法: %q", path, n, line)
		}

		if m := scenarioRe.FindStringSubmatch(line); m != nil {
			if cur < 0 {
				return nil, fmt.Errorf("%s:%d: Scenario 出现在任何 REQ 之前: %q", path, n, m[1])
			}
			reqs[cur].Scenarios = append(reqs[cur].Scenarios, Scenario{Name: m[1]})
			curScenario = len(reqs[cur].Scenarios) - 1
			continue
		}

		if m := gwtRe.FindStringSubmatch(line); m != nil && cur >= 0 && curScenario >= 0 {
			s := &reqs[cur].Scenarios[curScenario]
			switch m[1] {
			case "GIVEN":
				s.HasGiven = true
			case "WHEN":
				s.HasWhen = true
			case "THEN":
				s.HasThen = true
			}
		}
	}
	return reqs, sc.Err()
}
