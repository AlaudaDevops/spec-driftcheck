package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// record 记录一次写操作（创建/更新）的请求方法与路径。
type record struct {
	method string
	path   string
}

// newServer 返回一个模拟 GitHub Issues 评论 API 的服务器。
// listBody 是 GET 列表接口返回的 JSON 数组；写操作记录到 got。
func newServer(t *testing.T, listBody string, got *record) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(listBody))
			return
		}
		got.method = r.Method
		got.path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func TestUpsertComment_Create(t *testing.T) {
	var got record
	srv := newServer(t, `[]`, &got)
	defer srv.Close()

	err := UpsertComment(srv.URL, "o/r", 1, "tok", "<!-- m -->", "<!-- m -->\nbody")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost {
		t.Errorf("want POST, got %s", got.method)
	}
	if got.path != "/repos/o/r/issues/1/comments" {
		t.Errorf("want create path, got %s", got.path)
	}
}

func TestUpsertComment_Update(t *testing.T) {
	var got record
	srv := newServer(t, `[{"id":99,"body":"<!-- m -->\nold"}]`, &got)
	defer srv.Close()

	err := UpsertComment(srv.URL, "o/r", 1, "tok", "<!-- m -->", "<!-- m -->\nnew")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch {
		t.Errorf("want PATCH, got %s", got.method)
	}
	if got.path != "/repos/o/r/issues/comments/99" {
		t.Errorf("want update path, got %s", got.path)
	}
}

// TestUpsertComment_ListError 断言 GET 列表返回 401 时直接报错，
// 且不会发起任何写操作（POST/PATCH）。
func TestUpsertComment_ListError(t *testing.T) {
	var wrote bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		wrote = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := UpsertComment(srv.URL, "o/r", 1, "tok", "<!-- m -->", "<!-- m -->\nbody")
	if err == nil {
		t.Fatal("want error on 401 list response, got nil")
	}
	if wrote {
		t.Error("must not perform any write after list error")
	}
}

// TestUpsertComment_Paginates 断言 marker 在第二页时也能找到：
// 第一页返回 100 条无 marker 的评论，第二页含 marker(id=7)，应发起 PATCH .../comments/7。
func TestUpsertComment_Paginates(t *testing.T) {
	var page1, page2 strings.Builder
	page1.WriteString("[")
	for i := 0; i < 100; i++ {
		if i > 0 {
			page1.WriteString(",")
		}
		fmt.Fprintf(&page1, `{"id":%d,"body":"noise %d"}`, 1000+i, i)
	}
	page1.WriteString("]")
	page2.WriteString(`[{"id":7,"body":"<!-- m -->\nold"}]`)

	var got record
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("page") == "2" {
				_, _ = w.Write([]byte(page2.String()))
			} else {
				_, _ = w.Write([]byte(page1.String()))
			}
			return
		}
		got.method = r.Method
		got.path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := UpsertComment(srv.URL, "o/r", 1, "tok", "<!-- m -->", "<!-- m -->\nnew")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch {
		t.Errorf("want PATCH, got %s", got.method)
	}
	if got.path != "/repos/o/r/issues/comments/7" {
		t.Errorf("want PATCH on id=7, got %s", got.path)
	}
}
