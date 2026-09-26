//go:build !windows

package session

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// killProcessGroup termina de forma ordenada y garantizada el grupo de procesos en sistemas POSIX/Unix.
func killProcessGroup(pid int) error {
	if pid <= 0 {
		return errors.New("PID inválido")
	}

	// 1. Comprobar si el proceso está activo enviando señal 0
	if err := syscall.Kill(pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("el proceso con PID %d ya no se encuentra en ejecución", pid)
		}
	}

	// 2. Determinar el PGID (Process Group ID)
	target := pid
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 0 {
		target = -pgid
	} else {
		target = -pid
	}

	// 3. Enviar SIGTERM al grupo completo para que los agentes y servidores MCP hijos cierren limpiamente
	_ = syscall.Kill(target, syscall.SIGTERM)

	// 4. Periodo de gracia de 1.5 segundos para vaciar buffers y guardar estado
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err != nil {
			// Proceso y grupo finalizados exitosamente
			return nil
		}
	}

	// 5. Si continúa con vida, forzar SIGKILL al grupo y al PID
	_ = syscall.Kill(target, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)

	return nil
}
