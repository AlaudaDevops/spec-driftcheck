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

// crdManifest 是 CustomResourceDefinition manifest 中本工具关心的字段。
type crdManifest struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Names struct {
			Kind string `yaml:"kind"`
		} `yaml:"names"`
	} `yaml:"spec"`
}

// DiscoverCRDs 枚举仓库 yaml 中全部 CustomResourceDefinition 定义的 spec.names.kind。
// 用于反向哨兵：发现已定义但未被任何 anchors 登记的新 CRD。
func DiscoverCRDs(repoDir string) []string {
	seen := map[string]bool{}
	var kinds []string
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// 快筛：不含 CRD 关键字的文件跳过解析。
		if !strings.Contains(string(data), "kind: CustomResourceDefinition") {
			return nil
		}
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		for {
			var m crdManifest
			if err := dec.Decode(&m); err != nil {
				break // EOF 或单 doc 解析失败：已尽力，跳过余下
			}
			if m.Kind == "CustomResourceDefinition" && m.Spec.Names.Kind != "" && !seen[m.Spec.Names.Kind] {
				seen[m.Spec.Names.Kind] = true
				kinds = append(kinds, m.Spec.Names.Kind)
			}
		}
		return nil
	})
	return kinds
}
