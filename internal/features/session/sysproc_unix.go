//go:build !windows

package session

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
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
			// Restaurar el grupo de procesos padre al Foreground de la terminal
			_, _, _ = unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCSPGRP, uintptr(unsafe.Pointer(&parentPgid)))
		}
	}

	// En entornos no interactivos o tuberías (pipes)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return func() {}
}
