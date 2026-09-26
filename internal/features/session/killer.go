package session

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// ProcessKiller define la interfaz para terminar procesos en el sistema operativo.
type ProcessKiller interface {
	Kill(pid int) error
}

// OSProcessKiller implementa la terminación de procesos mediante señales POSIX.
type OSProcessKiller struct{}

// Kill envía SIGTERM al proceso y, si no finaliza, fuerza la terminación con SIGKILL.
func (k *OSProcessKiller) Kill(pid int) error {
	if pid <= 0 {
		return errors.New("PID inválido")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("no se encontró proceso con PID %d: %w", pid, err)
	}

	// Comprobar si el proceso está activo enviando señal 0
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("el proceso con PID %d ya no se encuentra en ejecución", pid)
	}

	// Señal SIGTERM para terminación ordenada
	_ = proc.Signal(syscall.SIGTERM)

	// Pausa breve de 100ms
	time.Sleep(100 * time.Millisecond)

	// Si continúa activo, forzar SIGKILL
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		_ = proc.Kill()
	}

	return nil
}
