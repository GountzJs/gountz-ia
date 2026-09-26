//go:build windows

package session

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// killProcessGroup termina de forma ordenada y garantizada el árbol de procesos en Windows usando taskkill.
func killProcessGroup(pid int) error {
	if pid <= 0 {
		return errors.New("PID inválido")
	}

	pidStr := strconv.Itoa(pid)

	// 1. Verificar si el proceso existe inspeccionando exactamente la columna PID
	checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	out, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("el proceso con PID %d ya no se encuentra en ejecución", pid)
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	records, err := r.ReadAll()
	found := false
	if err == nil {
		for _, rec := range records {
			if len(rec) >= 2 && strings.TrimSpace(rec[1]) == pidStr {
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("el proceso con PID %d ya no se encuentra en ejecución", pid)
	}

	// 2. En Windows, los procesos de consola (agentes de terminal y servidores MCP)
	// ignoran WM_CLOSE (taskkill sin /F). Se aplica /F /T para terminar el árbol de procesos de forma garantizada.
	_ = exec.Command("taskkill", "/F", "/T", "/PID", pidStr).Run()
	return nil
}
