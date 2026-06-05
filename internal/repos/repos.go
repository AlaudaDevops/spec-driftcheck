// Package repos 解析 repos.yaml 并按需浅克隆组件仓库。
package repos

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Repo 是一个组件仓库的克隆配置。
type Repo struct {
	URL    string `yaml:"url"`
	Branch string `yaml:"branch"`
	Local  bool   `yaml:"local"`
}

// File 是 repos.yaml。
type File struct {
	Repos map[string]Repo `yaml:"repos"`
}

// Load 读取 repos.yaml。
func Load(path string) (File, error) {
	var f File
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	return f, yaml.Unmarshal(data, &f)
}

// Dir 返回仓库的本地目录：local 仓库返回 localRoot，否则浅克隆到 workDir。
func Dir(name string, r Repo, localRoot, workDir string) (string, error) {
	if r.Local {
		return localRoot, nil
	}
	dir := filepath.Join(workDir, name)
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", r.Branch, r.URL, dir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("clone %s: %w", name, err)
	}
	return dir, nil
}
