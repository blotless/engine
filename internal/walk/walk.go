package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/blotless/engine/ports"
)

var defaultSkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"bin":          {},
	".idea":        {},
	".vscode":      {},
}

var defaultSkipFiles = map[string]struct{}{
	"license":      {},
	"licence":      {},
	"copying":      {},
	"notice":       {},
	"patents":      {},
	"go.sum":       {},
	"go.work.sum":  {},
	"coverage.txt": {},
	"coverage.out": {},
	".ds_store":    {},
}

// Options control filesystem walking.
type Options struct {
	Include      []string
	Exclude      []string
	MaxFileBytes int64
	SkipBinary   bool
}

// Walker walks local directories.
type Walker struct {
	Opts Options
}

func (w Walker) Walk(ctx context.Context, roots []string, fn ports.WalkFunc) error {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Stat(root)
		if err != nil {
			return fmt.Errorf("walk: stat %s: %w", root, err)
		}
		if !info.IsDir() {
			meta := ports.FileMeta{Path: root, RelPath: filepath.Base(root), Size: info.Size()}
			if !w.allow(meta.RelPath) {
				continue
			}
			if w.Opts.MaxFileBytes > 0 && meta.Size > w.Opts.MaxFileBytes {
				continue
			}
			if err := fn(ctx, meta); err != nil {
				return err
			}
			continue
		}
		base := root
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if _, skip := defaultSkipDirs[name]; skip && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			rel, relErr := filepath.Rel(base, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			if !w.allow(rel) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if w.Opts.MaxFileBytes > 0 && info.Size() > w.Opts.MaxFileBytes {
				return nil
			}
			return fn(ctx, ports.FileMeta{Path: path, RelPath: rel, Size: info.Size()})
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (w Walker) allow(rel string) bool {
	rel = filepath.ToSlash(rel)
	if skipName(filepath.Base(rel)) {
		return false
	}
	if len(w.Opts.Include) > 0 {
		ok := false
		for _, p := range w.Opts.Include {
			if matchGlob(p, rel) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, p := range w.Opts.Exclude {
		if matchGlob(p, rel) {
			return false
		}
	}
	return true
}

func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	if pattern == "" {
		return false
	}
	if ok, _ := filepath.Match(pattern, name); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(name)); ok {
			return true
		}
	}
	if strings.HasPrefix(pattern, "**/") {
		suf := strings.TrimPrefix(pattern, "**/")
		if matchGlob(suf, name) {
			return true
		}
		for i := 0; i < len(name); i++ {
			if name[i] == '/' && matchGlob(suf, name[i+1:]) {
				return true
			}
		}
	}
	if strings.Contains(pattern, "/**/") {
		parts := strings.SplitN(pattern, "/**/", 2)
		if strings.HasPrefix(name, parts[0]+"/") && matchGlob(parts[1], name[len(parts[0])+1:]) {
			return true
		}
		rest := name
		if strings.HasPrefix(name, parts[0]+"/") {
			rest = name[len(parts[0])+1:]
		}
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' && matchGlob(parts[1], rest[i+1:]) {
				return true
			}
		}
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return false
}

func skipName(name string) bool {
	n := strings.ToLower(name)
	if _, ok := defaultSkipFiles[n]; ok {
		return true
	}
	for _, p := range []string{"license", "licence"} {
		if n == p || strings.HasPrefix(n, p+".") {
			return true
		}
	}
	return false
}

var _ ports.Walker = Walker{}
