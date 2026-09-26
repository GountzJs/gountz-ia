//go:build windows

package session

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// killProcessGroup termina de forma ordenada y garantizada el árbol de procesos en Windows usando taskkill.
func killProcessGroup(pid int) error {
	if pid <= 0 {
		return errors.New("PID inválido")
	}

	// 1. Verificar si el proceso existe
	checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	out, err := checkCmd.Output()
	if err != nil || !strings.Contains(string(out), strconv.Itoa(pid)) {
		return fmt.Errorf("el proceso con PID %d ya no se encuentra en ejecución", pid)
	}

	// 2. Intentar terminación ordenada con taskkill /T /PID <pid>
	_ = exec.Command("taskkill", "/T", "/PID", strconv.Itoa(pid)).Run()

	// 3. Periodo de gracia de 1.5 segundos
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		check := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		chkOut, chkErr := check.Output()
		if chkErr != nil || !strings.Contains(string(chkOut), strconv.Itoa(pid)) {
			return nil
		}
	}

	// 4. Forzar terminación con /F /T
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	return nil
}
