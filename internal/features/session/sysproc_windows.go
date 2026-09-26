//go:build windows

package session

import (
	"os/exec"
)

func configureSysProcAttr(cmd *exec.Cmd) func() {
	return func() {}
}
