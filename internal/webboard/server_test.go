package webboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const testToken = "test-capability-not-a-real-secret"
const authority = "127.0.0.1:43210"

func webProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	p, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(p.Work, "1-open", "W-01-card.md"), "# W-01: Literal <script>\n\n- **Stage:** todo\n- **Updated:** 2026-11-14\n- **Next action:** design\n- **Where:** —\n\n## Goal\n<script>alert(1)</script>\n\n## Comments\n")
	return p
}

func request(h http.Handler, method, path, host, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+authority+path, nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAuthenticatedReadOnlyBoard(t *testing.T) {
	p := webProject(t)
	h := handler(p, authority, testToken)
	base := "/" + testToken + "/"
	w := request(h, "GET", base+"snapshot", authority, "http://"+authority)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "W-01") {
		t.Fatalf("snapshot: %d %s", w.Code, w.Body)
	}
	var snapshot struct {
		Board struct {
			Columns []any `json:"columns"`
		} `json:"board"`
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil || len(snapshot.Board.Columns) == 0 {
		t.Fatalf("snapshot shape: %v %s", err, w.Body)
	}
	for _, key := range []string{"Content-Security-Policy", "Cache-Control", "Referrer-Policy", "X-Content-Type-Options"} {
		if w.Header().Get(key) == "" {
			t.Errorf("missing %s", key)
		}
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS enabled")
	}
	w = request(h, "GET", base+"brief?id=W-01", authority, "")
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") || !strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
		t.Fatalf("literal brief: %d %s", w.Code, w.Body)
	}
	for _, asset := range []string{"", "sessions.js", "toasts.js", "render.js", "browser.js"} {
		w = request(h, "GET", base+asset, authority, "")
		if w.Code != 200 {
			t.Errorf("asset %q: %d", asset, w.Code)
		}
		if head := request(h, "HEAD", base+asset, authority, ""); head.Code != 200 || head.Body.Len() != 0 {
			t.Errorf("HEAD %q: %d %s", asset, head.Code, head.Body)
		}
	}
}

func TestHostOriginTokenAndMethodsAreConfined(t *testing.T) {
	h := handler(webProject(t), authority, testToken)
	base := "/" + testToken + "/"
	for _, tc := range []struct{ method, path, host, origin string }{
		{"GET", "/", authority, ""}, {"GET", "/wrong/snapshot", authority, ""},
		{"GET", base + "snapshot", "evil.example:43210", ""},
		{"GET", base + "snapshot", "localhost:43210", ""},
		{"GET", base + "snapshot", authority, "http://evil.example"},
		{"GET", base + "snapshot", authority, "null"},
		{"POST", base + "snapshot", authority, ""}, {"OPTIONS", base + "snapshot", authority, ""},
		{"GET", base + "../snapshot", authority, ""}, {"GET", base + "%2e%2e/snapshot", authority, ""},
		{"GET", base + "assets.go", authority, ""}, {"GET", base + ".git/config", authority, ""},
		{"GET", base + "brief?id=../W-01", authority, ""},
		{"GET", base + "brief?id=W-02", authority, ""},
	} {
		w := request(h, tc.method, tc.path, tc.host, tc.origin)
		if w.Code < 400 || strings.Contains(w.Body.String(), "Literal") {
			t.Errorf("not confined: %+v => %d %s", tc, w.Code, w.Body)
		}
	}
}

func TestMultipleOriginsAreRejected(t *testing.T) {
	h := handler(webProject(t), authority, testToken)
	r := httptest.NewRequest("GET", "http://"+authority+"/"+testToken+"/snapshot", nil)
	r.Header.Add("Origin", "http://"+authority)
	r.Header.Add("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("ambiguous Origin accepted: %d", w.Code)
	}
}

func TestBriefCannotFollowEscapingSymlink(t *testing.T) {
	p := webProject(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	testutil.Write(t, outside, "# W-02: private outside\n\n- **Stage:** todo\n- **Updated:** 2026-11-14\n- **Where:** —\n- **Next action:** design\n\n## Comments\n")
	if err := os.Symlink(outside, filepath.Join(p.Work, "1-open", "W-02-private.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	w := request(handler(p, authority, testToken), "GET", "/"+testToken+"/brief?id=W-02", authority, "")
	if w.Code < 400 || strings.Contains(w.Body.String(), "private outside") {
		t.Fatalf("escaped: %d %s", w.Code, w.Body)
	}
}

func TestForegroundLifecycleAndBrowserFailure(t *testing.T) {
	p := webProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	finished := make(chan error, 1)
	var output, warnings bytes.Buffer
	go func() {
		finished <- Serve(ctx, p, Options{Out: &output, Err: &warnings, open: func(string) error { return errors.New("no desktop") }, ready: func(u string) { ready <- u }})
	}()
	var address string
	select {
	case address = <-ready:
	case err := <-finished:
		t.Fatalf("server exited before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server never ready")
	}
	if !strings.HasPrefix(address, "http://127.0.0.1:") {
		t.Fatalf("not loopback: %s", address)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	resp, err := client.Get(address + "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(data), "W-01") {
		t.Fatalf("live snapshot: %d %s", resp.StatusCode, data)
	}
	cancel()
	select {
	case err = <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown hung")
	}
	if !strings.Contains(output.String(), address) || !strings.Contains(warnings.String(), "no desktop") {
		t.Fatalf("launch output: %s / %s", &output, &warnings)
	}
	if resp, err = client.Get(address + "snapshot"); err == nil {
		resp.Body.Close()
		t.Fatal("server still running")
	}
}

func TestNoOpenBindFailureAndFreshCapabilities(t *testing.T) {
	p := webProject(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := Serve(context.Background(), p, Options{Port: listener.Addr().(*net.TCPAddr).Port, NoOpen: true}); err == nil {
		t.Fatal("occupied port succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = Serve(ctx, p, Options{NoOpen: true, open: func(string) error { t.Error("browser opened with --no-open"); return nil }, ready: func(string) { cancel() }})
	if err != nil {
		t.Fatal(err)
	}
	a, err := capability()
	if err != nil {
		t.Fatal(err)
	}
	b, err := capability()
	if err != nil || a == b || len(a) != 64 || len(b) != 64 {
		t.Fatalf("capabilities not fresh: %q %q %v", a, b, err)
	}
}

func TestRelocatedCardsWorktreeRefreshAndReadFailure(t *testing.T) {
	p := webProject(t)
	custom := filepath.Join(p.Root, "custom-cards")
	if err := os.Rename(p.Work, custom); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(p.Root, ".kanbanzai", "config.toml")
	text := "config_version = 1\n[project]\nwork = \"custom-cards\"\nowner = \"browser owner\"\n"
	testutil.Write(t, configFile, text)
	testutil.Git(t, p.Root, "add", ".")
	testutil.Git(t, p.Root, "commit", "-qm", "fixture")
	wt := filepath.Join(t.TempDir(), "worktree")
	testutil.Git(t, p.Root, "worktree", "add", "-b", "browser-test", wt)
	wp, err := config.Find(wt)
	if err != nil {
		t.Fatal(err)
	}
	h := handler(wp, authority, testToken)
	base := "/" + testToken + "/"
	if w := request(h, "GET", base+"snapshot", authority, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "browser owner") {
		t.Fatalf("worktree snapshot: %d %s", w.Code, w.Body)
	}
	brief := filepath.Join(custom, "1-open", "W-01-card.md")
	f, err := os.OpenFile(brief, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\nupdated externally\n")
	f.Close()
	if w := request(h, "GET", base+"brief?id=W-01", authority, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "updated externally") {
		t.Fatalf("fresh relocated brief: %d %s", w.Code, w.Body)
	}
	testutil.Write(t, configFile, "config_version = 999\n")
	if w := request(h, "GET", base+"snapshot", authority, ""); w.Code != 500 {
		t.Fatalf("failed read became good snapshot: %d %s", w.Code, w.Body)
	}
	testutil.Write(t, configFile, text)
	if w := request(h, "GET", base+"snapshot", authority, ""); w.Code != 200 {
		t.Fatalf("read did not recover: %d %s", w.Code, w.Body)
	}
}

func TestBrowserLaunchUsesArgumentsNotShell(t *testing.T) {
	url := "http://127.0.0.1:1234/private/"
	for _, goos := range []string{"linux", "darwin", "windows"} {
		name, args := browserCommand(goos, url)
		if name == "" || len(args) == 0 || args[len(args)-1] != url || name == "sh" || name == "cmd" {
			t.Fatalf("unsafe %s opener: %q %q", goos, name, args)
		}
	}
}
