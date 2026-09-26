package session

// ProcessKiller define la interfaz para terminar procesos en el sistema operativo.
type ProcessKiller interface {
	Kill(pid int) error
}

// OSProcessKiller implementa la terminación de procesos y sus grupos/árboles de procesos hijos.
type OSProcessKiller struct{}

// Kill termina de forma ordenada el proceso y todos sus subprocesos (grupo/árbol).
func (k *OSProcessKiller) Kill(pid int) error {
	return killProcessGroup(pid)
}
