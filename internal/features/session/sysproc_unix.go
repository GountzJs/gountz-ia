//go:build !windows

package session

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/mattn/go-isatty"
	"golang.org/x/sys/unix"
)

func isTerminal(fd int) bool {
	return isatty.IsTerminal(uintptr(fd))
}

func restoreTTY(fd int, parentPgid int) {
	signal.Ignore(syscall.SIGTTOU)
	defer signal.Reset(syscall.SIGTTOU)
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, parentPgid)
}

// configureSysProcAttr configura los atributos de proceso en sistemas Unix.
// Si stdin es una TTY interactiva, sitúa el grupo de procesos en el Foreground
// de la terminal para evitar señales SIGTTIN al leer de teclado y permitir la recepción de Ctrl+C.
func configureSysProcAttr(cmd *exec.Cmd) func() {
	fd := int(os.Stdin.Fd())
	if isTerminal(fd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid:    true,
			Foreground: true,
			Ctty:       fd,
		}
		parentPgid := syscall.Getpgrp()
		return func() {
			restoreTTY(fd, parentPgid)
		}
	}

	// En entornos no interactivos o tuberías (pipes)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return func() {}
}
