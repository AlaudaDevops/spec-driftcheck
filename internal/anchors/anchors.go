// Package anchors 解析 anchors.yaml 并校验路径锚点与 CRD 锚点。
package anchors

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// File 是一个能力域的 anchors.yaml。
type File struct {
	Capability string                `yaml:"capability"`
	Repos      map[string]RepoAnchor `yaml:"repos"`
}

// RepoAnchor 是单个仓库的锚点声明。
type RepoAnchor struct {
	Paths []string `yaml:"paths"`
	CRDs  []string `yaml:"crds"`
}

// Load 读取并解析 anchors.yaml。
func Load(path string) (File, error) {
	var f File
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	return f, yaml.Unmarshal(data, &f)
}

// ValidatePaths 返回在 repoDir 下匹配不到任何文件的 glob。
func ValidatePaths(repoDir string, patterns []string) []string {
	var bad []string
	for _, p := range patterns {
		matches, err := doublestar.Glob(os.DirFS(repoDir), p)
		if err != nil || len(matches) == 0 {
			bad = append(bad, p)
		}
	}
	return bad
}

// CRDDefined 在仓库中搜索 "kind: <name>"（yaml）或 "type <name> struct"（go）。
func CRDDefined(repoDir, name string) bool {
	needles := []string{"kind: " + name, "type " + name + " struct"}
	found := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipAll
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" && ext != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, n := range needles {
			if strings.Contains(string(data), n) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}
