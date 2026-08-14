package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/blotless/engine/ports"
)

func TestOllamaClassifyAndRewrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
		case "/api/generate":
			var req ollamaReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Format == "json" {
				_, _ = io.WriteString(w, `{"response":"{\"confidence\":\"likely\",\"rationale\":\"test\",\"agent\":\"none\"}"}`)
				return
			}
			_, _ = io.WriteString(w, `{"response":"rewritten fragment"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	o, err := New(Config{Mode: "ollama", Endpoint: srv.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	oll := o.(*Ollama)

	out, err := oll.Classify(context.Background(), ports.ClassifyIn{
		File:     "a.go",
		Fragment: "func New() {}",
		Context:  "New",
	})
	if err != nil || out.Confidence != "likely" {
		t.Fatalf("classify: err=%v out=%#v", err, out)
	}

	rw, err := oll.Rewrite(context.Background(), ports.RewriteIn{Fragment: "hello world"})
	if err != nil || rw.Rewrite != "rewritten fragment" {
		t.Fatalf("rewrite: err=%v rw=%#v", err, rw)
	}
}

func TestOllamaGenerateErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case "/api/pull":
			w.WriteHeader(http.StatusOK)
		case "/api/generate":
			w.WriteHeader(http.StatusTeapot)
			_, _ = io.WriteString(w, "fail")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	o, err := New(Config{Mode: "ollama", Endpoint: srv.URL, Model: "missing", Timeout: time.Second, PullTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	oll := o.(*Ollama)

	_, err = oll.generate(context.Background(), "prompt")
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("%v", err)
	}
}

func TestOllamaGetPostErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	o := &Ollama{cfg: Config{Endpoint: srv.URL, Timeout: time.Second}}
	_, err := o.listModels()
	if err == nil {
		t.Fatal("expected tags error")
	}
}

func TestOllamaClassifyBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
		case "/api/generate":
			_, _ = io.WriteString(w, `{"response":"not json at all"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	o, err := New(Config{Mode: "ollama", Endpoint: srv.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	out, err := o.(*Ollama).Classify(context.Background(), ports.ClassifyIn{Fragment: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Rationale != "" {
		t.Fatalf("%#v", out)
	}
}

func TestPingBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	if err := ping(srv.URL, 500*time.Millisecond); err == nil {
		t.Fatal("expected ping failure")
	}
}

func TestPingEventuallyReady(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := ping(srv.URL, 2*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestPidFileDefaultPath(t *testing.T) {
	os.Unsetenv("BLOTLESS_CACHE")
	if pidFile() == "" {
		t.Fatal("empty pid path")
	}
}

func TestOllamaConfigMode(t *testing.T) {
	pulled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
		case "/api/pull":
			pulled = true
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	o, err := New(Config{Mode: "ollama", Endpoint: srv.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	if pulled {
		t.Fatal("should not pull when model is present")
	}
	if o.Name() != "ollama" {
		t.Fatal(o.Name())
	}
}

func TestOllamaConfigPullsMissingModel(t *testing.T) {
	pulled := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case "/api/pull":
			var req pullReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			pulled = req.Name
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	o, err := New(Config{Mode: "ollama", Endpoint: srv.URL, Model: "phi-3-mini", Timeout: time.Second, PullTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	if pulled != "phi-3-mini" {
		t.Fatalf("pulled=%q", pulled)
	}
}

func TestOllamaConfigUnreachable(t *testing.T) {
	_, err := New(Config{Mode: "ollama", Endpoint: "http://127.0.0.1:1", Timeout: 200 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("got %v", err)
	}
}

func TestOllamaAutoMissingBinary(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	oldLook, oldProbe, oldWait := lookPath, probeURL, existingProbeWait
	t.Cleanup(func() {
		lookPath = oldLook
		probeURL = oldProbe
		existingProbeWait = oldWait
	})
	probeURL = "http://127.0.0.1:1"
	existingProbeWait = 80 * time.Millisecond
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }

	_, err := New(Config{Mode: "ollama", Timeout: 200 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "binary not found") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "ollama.com") {
		t.Fatalf("install hint missing: %v", err)
	}
}

func TestOllamaAutoReusesProbe(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	oldLook, oldProbe := lookPath, probeURL
	t.Cleanup(func() {
		lookPath = oldLook
		probeURL = oldProbe
	})
	probeURL = srv.URL
	lookPath = func(string) (string, error) {
		t.Fatal("should not look up ollama when a server is already up")
		return "", exec.ErrNotFound
	}

	o, err := New(Config{Mode: "ollama", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	got, ok := o.(*Ollama)
	if !ok || got.owned {
		t.Fatalf("expected unmanaged client, got %#v", o)
	}
	if got.cfg.Endpoint != strings.TrimRight(srv.URL, "/") {
		t.Fatalf("endpoint=%s", got.cfg.Endpoint)
	}
}

func TestOllamaAutoReusesExistingManaged(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	oldLook, oldProbe, oldWait, oldHost := lookPath, probeURL, existingProbeWait, managedHost
	t.Cleanup(func() {
		lookPath = oldLook
		probeURL = oldProbe
		existingProbeWait = oldWait
		managedHost = oldHost
	})
	probeURL = "http://127.0.0.1:1"
	existingProbeWait = 80 * time.Millisecond
	managedHost = strings.TrimPrefix(srv.URL, "http://")
	lookPath = func(string) (string, error) {
		t.Fatal("should reuse managed host without starting ollama")
		return "", exec.ErrNotFound
	}

	o, err := New(Config{Mode: "ollama", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	got := o.(*Ollama)
	if got.owned || got.cfg.Endpoint != strings.TrimRight(srv.URL, "/") {
		t.Fatalf("owned=%v endpoint=%s", got.owned, got.cfg.Endpoint)
	}
}

func TestOllamaAutoStartsManagedHost(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
			return
		}
		http.NotFound(w, r)
	})}
	t.Cleanup(func() { _ = srv.Close() })

	oldLook, oldProbe, oldWait, oldStart, oldHost := lookPath, probeURL, existingProbeWait, startServe, managedHost
	t.Cleanup(func() {
		lookPath = oldLook
		probeURL = oldProbe
		existingProbeWait = oldWait
		startServe = oldStart
		managedHost = oldHost
	})
	probeURL = "http://127.0.0.1:1"
	existingProbeWait = 80 * time.Millisecond
	lookPath = func(string) (string, error) { return "/usr/bin/ollama", nil }
	managedHost = fmt.Sprintf("127.0.0.1:%d", port)
	started := ""
	startServe = func(bin, host string) (*exec.Cmd, context.CancelFunc, error) {
		if bin != "/usr/bin/ollama" {
			t.Fatalf("bin=%s", bin)
		}
		started = host
		go func() { _ = srv.Serve(ln) }()
		return nil, func() {}, nil
	}

	o, err := New(Config{Mode: "ollama", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	want := fmt.Sprintf("127.0.0.1:%d", port)
	if started != want {
		t.Fatalf("started host %q want %q", started, want)
	}
	got := o.(*Ollama)
	if !got.owned || got.cfg.Endpoint != "http://"+want {
		t.Fatalf("owned=%v endpoint=%s", got.owned, got.cfg.Endpoint)
	}
}

func TestOllamaCloseKillsTree(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	killed := 0
	old := killTree
	t.Cleanup(func() { killTree = old })
	killTree = func(pid int) { killed = pid }
	o := &Ollama{owned: true, pid: 77, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if killed != 77 || o.owned {
		t.Fatalf("killed=%d owned=%v", killed, o.owned)
	}
}

func TestOllamaReclaimStopsPrevious(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	if err := writePidFile(4242, "http://127.0.0.1:12434"); err != nil {
		t.Fatal(err)
	}
	killed := 0
	oldKill, oldLook, oldProbe, oldWait, oldStart, oldHost := killTree, lookPath, probeURL, existingProbeWait, startServe, managedHost
	t.Cleanup(func() {
		killTree = oldKill
		lookPath = oldLook
		probeURL = oldProbe
		existingProbeWait = oldWait
		startServe = oldStart
		managedHost = oldHost
	})
	killTree = func(pid int) { killed = pid }
	probeURL = "http://127.0.0.1:1"
	existingProbeWait = 80 * time.Millisecond
	lookPath = func(string) (string, error) { return "/usr/bin/ollama", nil }
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": DefaultOllamaModel}},
			})
			return
		}
		http.NotFound(w, r)
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	managedHost = fmt.Sprintf("127.0.0.1:%d", port)
	startServe = func(bin, host string) (*exec.Cmd, context.CancelFunc, error) {
		return nil, func() {}, nil
	}

	o, err := New(Config{Mode: "ollama", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = o.Close() })
	if killed != 4242 {
		t.Fatalf("killed=%d", killed)
	}
}

func TestPidFileRoundTrip(t *testing.T) {
	t.Setenv("BLOTLESS_CACHE", t.TempDir())
	if err := writePidFile(99, "http://127.0.0.1:12434"); err != nil {
		t.Fatal(err)
	}
	pid, ep, ok := readPidFile()
	if !ok || pid != 99 || ep != "http://127.0.0.1:12434" {
		t.Fatalf("pid=%d ep=%s ok=%v", pid, ep, ok)
	}
	clearPidFile()
	if _, _, ok := readPidFile(); ok {
		t.Fatal("pidfile still present")
	}
}

func TestHasModel(t *testing.T) {
	if !hasModel([]string{DefaultOllamaModel}, DefaultOllamaModel) {
		t.Fatal("exact")
	}
	if hasModel([]string{"llama3:8b"}, DefaultOllamaModel) {
		t.Fatal("other")
	}
}

func TestFreePort(t *testing.T) {
	p, err := freePort()
	if err != nil || p <= 0 {
		t.Fatalf("port=%d err=%v", p, err)
	}
}

func TestOllamaListModelsBothFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{{"name": "a", "model": "b"}},
		})
	}))
	t.Cleanup(srv.Close)
	o := &Ollama{cfg: Config{Endpoint: srv.URL, Timeout: time.Second}}
	names, err := o.listModels()
	if err != nil || len(names) != 2 {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

func TestHasModelTagSuffix(t *testing.T) {
	if !hasModel([]string{"llama3:8b@digest"}, "llama3:8b") {
		t.Fatal("tag suffix")
	}
}

func TestOllamaPostNetworkError(t *testing.T) {
	o := &Ollama{cfg: Config{Endpoint: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond}}
	_, err := o.post(context.Background(), "/api/generate", []byte(`{}`), time.Second)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestOllamaEnsureModelAlreadyPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "custom-model"}},
			})
		}
	}))
	t.Cleanup(srv.Close)
	o := &Ollama{cfg: Config{Endpoint: srv.URL, Model: "custom-model", Timeout: time.Second}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := o.ensureModel(); err != nil {
		t.Fatal(err)
	}
}

func TestKillProcessTreeNoPanic(t *testing.T) {
	killProcessTree(99999999)
}

func TestStartOllamaServe(t *testing.T) {
	bin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not found")
	}
	cmd, cancel, err := startOllamaServe(bin, "127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
