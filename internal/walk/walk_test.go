package walk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blotless/engine/ports"
)

func TestWalkIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".git", "x"), []byte("no"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644)
	w := Walker{Opts: Options{Include: []string{"**/*.go"}}}
	var paths []string
	err := w.Walk(context.Background(), []string{dir}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, filepath.Base(meta.Path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "a.go" {
		t.Fatalf("%v", paths)
	}
}

func TestSkipLicenseAndSum(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("mit"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "go.sum"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok"), 0o644)
	w := Walker{}
	var paths []string
	err := w.Walk(context.Background(), []string{dir}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, filepath.Base(meta.Path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "ok.go" {
		t.Fatalf("%v", paths)
	}
}

func TestExcludeTestFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "pkg"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "a.go"), []byte("package a"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "a_test.go"), []byte("package a"), 0o644)
	w := Walker{Opts: Options{Exclude: []string{"*_test.go"}}}
	var paths []string
	err := w.Walk(context.Background(), []string{dir}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, filepath.Base(meta.Path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "a.go" {
		t.Fatalf("%v", paths)
	}
}

func TestMatchGlob(t *testing.T) {
	if !matchGlob("**/*.go", "pkg/a.go") {
		t.Fatal("**/*.go")
	}
	if matchGlob("**/*.go", "a.txt") {
		t.Fatal("should not match txt")
	}
	if !matchGlob("*_test.go", "engine/stamp_test.go") {
		t.Fatal("basename *_test.go")
	}
	if !matchGlob("testdata/**", "testdata/foo/a.go") {
		t.Fatal("testdata/**")
	}
	if matchGlob("*_test.go", "stamp.go") {
		t.Fatal("non-test")
	}
}

func TestWalkSingleFileAndMaxBytes(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "ok.go")
	big := filepath.Join(dir, "big.go")
	if err := os.WriteFile(small, []byte("package ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(big, []byte("package big\n"+string(make([]byte, 64))), 0o644); err != nil {
		t.Fatal(err)
	}
	w := Walker{Opts: Options{MaxFileBytes: 20}}
	var paths []string
	err := w.Walk(context.Background(), []string{small, big}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, filepath.Base(meta.Path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "ok.go" {
		t.Fatalf("%v", paths)
	}
}

func TestWalkDefaultRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	var n int
	w := Walker{}
	if err := w.Walk(context.Background(), nil, func(context.Context, ports.FileMeta) error {
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("files=%d", n)
	}
}

func TestWalkContextCancel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Walker{}.Walk(ctx, []string{dir}, func(context.Context, ports.FileMeta) error { return nil })
	if err == nil {
		t.Fatal("expected cancel")
	}
}

func TestWalkSkipNodeModules(t *testing.T) {
	dir := t.TempDir()
	nm := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nm, "a.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := Walker{}
	var paths []string
	if err := w.Walk(context.Background(), []string{dir}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, filepath.Base(meta.Path))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "ok.go" {
		t.Fatalf("%v", paths)
	}
}

func TestMatchGlobMiddle(t *testing.T) {
	if !matchGlob("src/**/test/*.go", "src/a/test/b.go") {
		t.Fatal("middle glob")
	}
	if matchGlob("", "a.go") {
		t.Fatal("empty pattern")
	}
}

func TestSkipLicensePrefix(t *testing.T) {
	if !skipName("LICENSE.md") {
		t.Fatal("license.md")
	}
}

func TestWalkMissing(t *testing.T) {
	err := Walker{}.Walk(context.Background(), []string{filepath.Join(t.TempDir(), "nope")}, func(context.Context, ports.FileMeta) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWalkSymlinkFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	link := filepath.Join(dir, "link.go")
	if err := os.WriteFile(target, []byte("package target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	w := Walker{}
	var paths []string
	err := w.Walk(context.Background(), []string{link}, func(_ context.Context, meta ports.FileMeta) error {
		paths = append(paths, meta.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("%v", paths)
	}
}
