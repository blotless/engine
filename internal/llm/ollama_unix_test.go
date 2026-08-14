//go:build unix

package llm

import (
	"os/exec"
	"testing"
)

func TestConfigureServeCmd(t *testing.T) {
	cmd := exec.Command("true")
	configureServeCmd(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("setpgid not configured")
	}
}
