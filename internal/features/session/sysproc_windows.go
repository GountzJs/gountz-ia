//go:build windows

package session

import (
	"os/exec"
)

func configureSysProcAttr(cmd *exec.Cmd) {
	// En Windows no se requiere Setpgid; el control de árbol se realiza vía taskkill /T o Job Objects
}
