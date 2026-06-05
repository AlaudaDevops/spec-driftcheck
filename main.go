// driftcheck 校验业务价值 spec 与各组件仓库的一致性。
//
//	driftcheck init   --plugin-name <name> --spec-repo <owner/repo> [--out spec] [--tool-image <image>]
//	driftcheck check  --spec-dir <dir> --work-dir <dir> --local-repo-root <dir>
//	driftcheck notice --repo-name <name> --changed-files <file> --spec-dir <dir> --github-repo <owner/repo> --pr <n>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yhuan123/spec-driftcheck/internal/github"
	"github.com/yhuan123/spec-driftcheck/internal/notice"
	"github.com/yhuan123/spec-driftcheck/internal/report"
	"github.com/yhuan123/spec-driftcheck/internal/runner"
	"github.com/yhuan123/spec-driftcheck/internal/scaffold"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "notice":
		err = runNotice(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: driftcheck <init|check|notice> [flags]")
	os.Exit(2)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	pluginName := fs.String("plugin-name", "", "插件名（如 gitlab）")
	specRepo := fs.String("spec-repo", "", "spec 所在主仓库 owner/repo")
	out := fs.String("out", "spec", "输出目录")
	toolImage := fs.String("tool-image", "ghcr.io/yhuan123/spec-driftcheck:latest", "driftcheck 工具镜像")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pluginName == "" || *specRepo == "" {
		return fmt.Errorf("--plugin-name 与 --spec-repo 必填")
	}
	written, err := scaffold.Render(*out, scaffold.Params{
		PluginName: *pluginName,
		SpecRepo:   *specRepo,
		ToolImage:  *toolImage,
	})
	if err != nil {
		return err
	}
	for _, f := range written {
		fmt.Println("created:", f)
	}
	fmt.Printf("\n脚手架就绪。下一步：按 %s/blob/main/docs/playbook.md 起草能力域。\n", "https://github.com/yhuan123/spec-driftcheck")
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	specDir := fs.String("spec-dir", "", "spec 目录（含 capabilities/ 与 sync/）")
	workDir := fs.String("work-dir", "/tmp/driftcheck-repos", "跨仓库克隆工作目录")
	localRoot := fs.String("local-repo-root", "", "local 仓库（tektoncd-operator）根目录")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *specDir == "" || *localRoot == "" {
		return fmt.Errorf("--spec-dir 与 --local-repo-root 必填")
	}
	findings, err := runner.Run(*specDir, *workDir, *localRoot)
	if err != nil {
		return err
	}
	fmt.Print(report.Render(findings))
	if len(findings) > 0 {
		os.Exit(1)
	}
	return nil
}

func runNotice(args []string) error {
	fs := flag.NewFlagSet("notice", flag.ExitOnError)
	repoName := fs.String("repo-name", "", "anchors.yaml 中的仓库键名")
	changedFile := fs.String("changed-files", "", "变更文件清单（每行一个路径）")
	specDir := fs.String("spec-dir", "", "spec 目录")
	ghRepo := fs.String("github-repo", "", "owner/repo")
	pr := fs.Int("pr", 0, "PR 编号")
	apiBase := fs.String("api-base", "https://api.github.com", "GitHub API 地址")
	specLink := fs.String("spec-link", "", "评论中 spec 目录的浏览地址（可选）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	data, err := os.ReadFile(*changedFile)
	if err != nil {
		return err
	}
	changed := strings.Fields(string(data))
	hits, err := notice.Scan(*specDir, *repoName, changed)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Println("未触碰 spec 锚点。")
		return nil
	}
	const marker = "<!-- spec-drift-notice -->"
	body := notice.RenderComment(hits, marker, *specLink)
	fmt.Print(body)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" || *ghRepo == "" || *pr == 0 {
		fmt.Println("(无 GITHUB_TOKEN/--github-repo/--pr，跳过评论，仅打印)")
		return nil
	}
	return github.UpsertComment(*apiBase, *ghRepo, *pr, token, marker, body)
}
