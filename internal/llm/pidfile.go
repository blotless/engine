package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func pidFile() string {
	if override := strings.TrimSpace(os.Getenv("BLOTLESS_CACHE")); override != "" {
		return filepath.Join(override, "ollama.pid")
	}
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "blotless", "ollama.pid")
}

func writePidFile(pid int, endpoint string) error {
	if pid <= 0 {
		return nil
	}
	path := pidFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("%d\n%s\n", pid, strings.TrimSpace(endpoint))
	return os.WriteFile(path, []byte(body), 0o600)
}

func readPidFile() (pid int, endpoint string, ok bool) {
	b, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, "", false
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 {
		return 0, "", false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		return 0, "", false
	}
	if len(lines) > 1 {
		endpoint = strings.TrimSpace(lines[1])
	}
	return pid, endpoint, true
}

func clearPidFile() {
	_ = os.Remove(pidFile())
}
