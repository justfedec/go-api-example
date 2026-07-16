package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startServerExample builds md, starts it with PORT set to a free port, and
// waits until the port accepts connections. The process is killed when the
// test finishes.
func startServerExample(t *testing.T, md string) (base string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "server")
	if err := Build(md, Options{Out: bin}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting server: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		if s := stderr.String(); s != "" {
			t.Logf("server stderr:\n%s", s)
		}
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for deadline := time.Now().Add(10 * time.Second); ; {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never became ready; stderr:\n%s", stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "http://" + addr
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return resp.StatusCode, string(b)
}

func TestHTTPServerEndToEnd(t *testing.T) {
	base := startServerExample(t, filepath.Join("testdata", "webserver.md"))

	if code, body := get(t, base+"/ping"); code != 200 || body != "pong" {
		t.Errorf("GET /ping = %d %q, want 200 \"pong\"", code, body)
	}

	resp, err := http.Post(base+"/echo?q=7", "text/plain", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatalf("POST /echo: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var echo struct{ Method, Body, Q string }
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("decoding /echo: %v", err)
	}
	if echo.Method != "POST" || echo.Body != `{"a":1}` || echo.Q != "7" {
		t.Errorf("echo = %+v", echo)
	}

	if code, body := get(t, base+"/nope"); code != 404 || body != "not found: /nope" {
		t.Errorf("GET /nope = %d %q", code, body)
	}
}

// TestTodoAPIExample drives examples/todo-api.md through a full CRUD cycle.
func TestTodoAPIExample(t *testing.T) {
	base := startServerExample(t, filepath.Join("..", "..", "examples", "todo-api.md"))
	do := func(method, path, body string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	if code, body := do("GET", "/todos", ""); code != 200 || body != "[]" {
		t.Fatalf("initial list = %d %q, want 200 []", code, body)
	}
	if code, body := do("POST", "/todos", `{"title": "ship \"v2\""}`); code != 201 ||
		body != `{"id": 1, "title": "ship \"v2\"", "completed": false}` {
		t.Fatalf("create = %d %q", code, body)
	}
	if code, body := do("POST", "/todos", `{"nope": 1}`); code != 400 || !strings.Contains(body, "title") {
		t.Fatalf("bad create = %d %q", code, body)
	}
	if code, body := do("PATCH", "/todos/1", ""); code != 200 || !strings.Contains(body, `"completed": true`) {
		t.Fatalf("toggle = %d %q", code, body)
	}
	if code, body := do("GET", "/todos", ""); code != 200 ||
		body != `[{"id": 1, "title": "ship \"v2\"", "completed": true}]` {
		t.Fatalf("list = %d %q", code, body)
	}
	if code, _ := do("PATCH", "/todos/99", ""); code != 404 {
		t.Fatalf("missing todo = %d, want 404", code)
	}
	if code, _ := do("DELETE", "/todos/1", ""); code != 204 {
		t.Fatalf("delete = %d, want 204", code)
	}
	if code, body := do("GET", "/todos", ""); code != 200 || body != "[]" {
		t.Fatalf("final list = %d %q, want 200 []", code, body)
	}
	if code, body := do("GET", "/health", ""); code != 200 || !strings.Contains(body, `"todos": 0`) {
		t.Fatalf("health = %d %q", code, body)
	}
}

// TestUnansweredRequestPanics checks the server guard: pulling a second
// request off the queue before answering the first is a clear panic, not a
// silent deadlock.
func TestUnansweredRequestPanics(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "server")
	if err := Build(filepath.Join("testdata", "unanswered.md"), Options{Out: bin}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for deadline := time.Now().Add(10 * time.Second); ; {
		if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up:\n%s", stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Two requests: the first is taken and never answered, the second makes
	// http.next panic. Fire both without waiting for a response body.
	client := &http.Client{Timeout: time.Second}
	go client.Get("http://" + addr + "/one")
	time.Sleep(200 * time.Millisecond)
	go client.Get("http://" + addr + "/two")

	err = cmd.Wait()
	if err == nil {
		t.Fatal("expected the server to exit nonzero")
	}
	if !strings.Contains(stderr.String(), "the previous request was never answered") {
		t.Errorf("stderr missing the guard message:\n%s", stderr.String())
	}
}

func TestHTTPClientEndToEnd(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello":
			io.WriteString(w, "hi")
		case "/items":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
				w.WriteHeader(400)
				return
			}
			w.WriteHeader(201)
			io.WriteString(w, `{"id": 7}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()
	t.Setenv("TEST_URL", fake.URL)

	var stdout, stderr bytes.Buffer
	code, err := Run(filepath.Join("testdata", "webclient.md"), Options{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr.String())
	}
	want := "true 200\nhi\n201 7\nfalse 0\n"
	if stdout.String() != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", stdout.String(), want)
	}
}

func TestLLMAskEndToEnd(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s, want /v1/messages", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if k := r.Header.Get("x-api-key"); k != "test-key" {
			t.Errorf("x-api-key = %q", k)
		}
		if v := r.Header.Get("anthropic-version"); v != "2023-06-01" {
			t.Errorf("anthropic-version = %q", v)
		}
		var req struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			System    string `json:"system"`
			Messages  []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		if req.Model != "claude-opus-4-8" || req.MaxTokens != 16000 || req.System != "You are terse." {
			t.Errorf("request = %+v", req)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "What is Inkdown?" {
			t.Errorf("messages = %+v", req.Messages)
		}
		io.WriteString(w, `{"stop_reason": "end_turn", "content": [{"type": "text", "text": "A literate language."}]}`)
	}))
	defer fake.Close()
	t.Setenv("ANTHROPIC_BASE_URL", fake.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	var stdout, stderr bytes.Buffer
	code, err := Run(filepath.Join("testdata", "llmask.md"), Options{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr.String())
	}
	if want := "A literate language.\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

// TestReviewRegressions pins fixes for defects found in the v2 code review.
func TestReviewRegressions(t *testing.T) {
	t.Run("exit terminates a value-returning function", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		code, err := Run(filepath.Join("testdata", "exitfn.md"), Options{Stdout: &stdout, Stderr: &stderr})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if code != 3 {
			t.Errorf("exit code = %d, want 3 (stderr: %s)", code, stderr.String())
		}
		if stdout.String() != "7\n" {
			t.Errorf("stdout = %q, want \"7\\n\"", stdout.String())
		}
		if !strings.Contains(stderr.String(), "boom") {
			t.Errorf("stderr missing eprint output: %q", stderr.String())
		}
	})

	t.Run("json numbers keep their literal form", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		code, err := Run(filepath.Join("testdata", "jsonnums.md"), Options{Stdout: &stdout, Stderr: &stderr})
		if err != nil || code != 0 {
			t.Fatalf("Run: %v, code %d, stderr:\n%s", err, code, stderr.String())
		}
		want := "1000001\n1721000000\n9007199254740993\n3.5\n"
		if stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	})

	t.Run("underscore is an ordinary identifier", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		code, err := Run(filepath.Join("testdata", "underscore.md"), Options{Stdout: &stdout, Stderr: &stderr})
		if err != nil || code != 0 {
			t.Fatalf("Run: %v, code %d, stderr:\n%s", err, code, stderr.String())
		}
		if stdout.String() != "5\n" {
			t.Errorf("stdout = %q, want \"5\\n\"", stdout.String())
		}
	})

	t.Run("error binding recovers instead of panicking", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		code, err := Run(filepath.Join("testdata", "errbind.md"), Options{Stdout: &stdout, Stderr: &stderr})
		if err != nil || code != 0 {
			t.Fatalf("Run: %v, code %d, stderr:\n%s", err, code, stderr.String())
		}
		// The last line's exact Go json error text varies by version, so
		// match the stable prefix.
		prefix := "42 true\n" +
			"0 int(): cannot parse \"oops\" as an integer\n" +
			"3 true\n" +
			"json.get: no value at path \"nope\"\n" +
			"2 true\n" +
			"json.get: the document is not valid JSON:"
		if !strings.HasPrefix(stdout.String(), prefix) {
			t.Errorf("stdout:\n%q\ndoes not start with:\n%q", stdout.String(), prefix)
		}
	})
}

func TestLLMAskRefusal(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"stop_reason": "refusal", "content": [], "stop_details": {"explanation": "out of scope"}}`)
	}))
	defer fake.Close()
	t.Setenv("ANTHROPIC_BASE_URL", fake.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	var stdout, stderr bytes.Buffer
	code, err := Run(filepath.Join("testdata", "llmask.md"), Options{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code == 0 {
		t.Error("expected a nonzero exit code on refusal")
	}
	if out := stderr.String(); !strings.Contains(out, "declined") || !strings.Contains(out, "out of scope") {
		t.Errorf("stderr missing refusal message:\n%s", out)
	}
}
