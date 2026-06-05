// Package github 提供 PR 评论的最小客户端。
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// UpsertComment 在 PR 上创建或更新含 marker 的评论。
func UpsertComment(apiBase, repo string, pr int, token, marker, body string) error {
	method, url := "POST", fmt.Sprintf("%s/repos/%s/issues/%d/comments", apiBase, repo, pr)
	const perPage = 100
	for page := 1; ; page++ {
		listURL := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=%d&page=%d", apiBase, repo, pr, perPage, page)
		req, _ := http.NewRequest("GET", listURL, nil)
		req.Header.Set("Authorization", "token "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			resp.Body.Close()
			return fmt.Errorf("github list comments: HTTP %d", resp.StatusCode)
		}
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()
		found := false
		for _, c := range comments {
			if strings.Contains(c.Body, marker) {
				method, url = "PATCH", fmt.Sprintf("%s/repos/%s/issues/comments/%d", apiBase, repo, c.ID)
				found = true
				break
			}
		}
		if found || len(comments) < perPage {
			break
		}
	}

	payload, _ := json.Marshal(map[string]string{"body": body})
	req2, _ := http.NewRequest(method, url, bytes.NewReader(payload))
	req2.Header.Set("Authorization", "token "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		return fmt.Errorf("github comment %s %s: HTTP %d", method, url, resp2.StatusCode)
	}
	return nil
}
