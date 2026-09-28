package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gz-ia/internal/features/session"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/updater"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Client gestiona la interfaz interactiva en terminal (TUI) de gz-ia.
type Client struct {
	service        session.Service
	runner         session.Runner
	store          session.Store
	killer         session.ProcessKiller
	workspace      workspace.Provider
	updaterService updater.Service
	toolingService tooling.Service
	vaultService   vault.Service
	in             io.Reader
	out            io.Writer
}

// New crea un nuevo cliente interactivo TUI.
func New(runner session.Runner, store session.Store, killer ...session.ProcessKiller) *Client {
	var r session.Runner = runner
	if r == nil {
		r = &session.OSRunner{}
	}
	var k session.ProcessKiller
	if len(killer) > 0 && killer[0] != nil {
		k = killer[0]
	} else {
		k = &session.OSProcessKiller{}
	}
	return &Client{
		runner:    r,
		store:     store,
		killer:    k,
		workspace: workspace.NewDefaultProvider(),
	}
}

// NewWithService crea un nuevo cliente interactivo TUI a partir de un Service existente.
func NewWithService(svc session.Service) *Client {
	return &Client{
		service:   svc,
		workspace: workspace.NewDefaultProvider(),
	}
}

// WithService permite inyectar una instancia de Service.
func (c *Client) WithService(svc session.Service) *Client {
	c.service = svc
	return c
}

// WithWorkspace permite inyectar un proveedor de workspace personalizado.
func (c *Client) WithWorkspace(ws workspace.Provider) *Client {
	c.workspace = ws
	return c
}

// WithUpdater permite inyectar una instancia del servicio de actualización.
func (c *Client) WithUpdater(u updater.Service) *Client {
	c.updaterService = u
	return c
}

// WithTooling permite inyectar una instancia del servicio de tooling modular.
func (c *Client) WithTooling(t tooling.Service) *Client {
	c.toolingService = t
	return c
}

// WithVault permite inyectar una instancia del servicio de vault de secretos.
func (c *Client) WithVault(v vault.Service) *Client {
	c.vaultService = v
	return c
}

func (c *Client) getVault(workDir string) vault.Service {
	if c.vaultService != nil {
		return c.vaultService
	}
	return vault.NewService(workDir)
}

func (c *Client) getToolingService(workDir string) tooling.Service {
	if c.toolingService != nil {
		return c.toolingService
	}
	return tooling.NewService("", workDir)
}

type singleByteReader struct {
	r io.Reader
}

func (s *singleByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return s.r.Read(p[:1])
}

// WithIO permite configurar lectores y escritores personalizados para tests o I/O redirigido.
func (c *Client) WithIO(in io.Reader, out io.Writer) *Client {
	if in != nil {
		c.in = &singleByteReader{r: in}
	} else {
		c.in = nil
	}
	c.out = out
	return c
}

func (c *Client) getIn() io.Reader {
	if c.in != nil {
		return c.in
	}
	return os.Stdin
}

func (c *Client) getOut() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *Client) getUpdater() updater.Service {
	if c.updaterService != nil {
		return c.updaterService
	}
	return updater.NewService()
}

func (c *Client) getWorkspace() workspace.Provider {
	if c.workspace != nil {
		return c.workspace
	}
	return workspace.NewDefaultProvider()
}

func (c *Client) getService(workDir string) session.Service {
	if c.service != nil {
		return c.service
	}
	var opts []session.Option
	if c.runner != nil {
		opts = append(opts, session.WithRunner(c.runner))
	}
	if c.store != nil {
		opts = append(opts, session.WithStore(c.store))
	}
	if c.killer != nil {
		opts = append(opts, session.WithKiller(c.killer))
	}
	if c.workspace != nil {
		opts = append(opts, session.WithWorkspace(c.workspace))
	}
	return session.NewService(workDir, opts...)
}

// Run ejecuta la presentación inicial y el menú interactivo principal.
func (c *Client) Run() error {
	icons := GetIcons()

	// 1. Mostrar banner estilo Fastfetch al iniciar
	fmt.Println(RenderFastfetchBanner())

	for {
		var action string
		menuTitle := fmt.Sprintf("%s  gz-ia — Base Agéntica", icons.Robot)
		menuDesc := fmt.Sprintf("%s Selecciona una opción y presiona Enter:", icons.Arrow)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(menuTitle).
					Description(menuDesc).
					Options(
						huh.NewOption(fmt.Sprintf("%s  Iniciar nuevo chat con agente", icons.Rocket), "start_chat"),
						huh.NewOption(fmt.Sprintf("%s  Sesiones activas e historial", icons.Worktree), "sessions"),
						huh.NewOption(fmt.Sprintf("%s  Gestionar Vault de Secretos / Entorno", icons.Lock), "vault"),
						huh.NewOption(fmt.Sprintf("%s  Actualizar gz-ia (update)", icons.Sync), "update"),
						huh.NewOption(fmt.Sprintf("%s  Cerrar / Salir", icons.Exit), "exit"),
					).
					Value(&action),
			),
		).WithTheme(CustomHuhTheme())

		if c.in != nil {
			form = form.WithInput(c.in).WithAccessible(true)
		}
		if c.out != nil {
			form = form.WithOutput(c.out)
		}

		if err := form.Run(); err != nil || action == "exit" {
			farewell := lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true).
				Render(fmt.Sprintf("\n%s ¡Hasta luego! Sesión de gz-ia finalizada con éxito.\n", icons.Sparkle))
			fmt.Println(farewell)
			return nil
		}

		switch action {
		case "start_chat":
			_ = c.handleNewChat(icons)
		case "sessions":
			_ = c.handleSessions(icons)
		case "vault":
			_ = c.handleVault(icons)
		case "update":
			_ = c.handleUpdate(icons)
		}
	}
}

// handleNewChat solicita la selección del agente y su nivel de permisos antes de iniciar la sesión.
func (c *Client) handleNewChat(icons IconSet) error {
	var selectedAgent string
	firstAvailable := session.FirstAvailableDriver()
	if firstAvailable != nil {
		selectedAgent = firstAvailable.ID()
	} else {
		selectedAgent = "back"
	}

	drivers := session.ListDrivers()
	agentOptions := make([]huh.Option[string], 0, len(drivers)+1)
	for _, d := range drivers {
		var label string
		if d.IsAvailable() {
			label = fmt.Sprintf("[✓] %s — Listo para usar", d.DisplayName())
		} else {
			label = fmt.Sprintf("[✗] %s — No instalado (no encontrado en PATH)", d.DisplayName())
		}
		agentOptions = append(agentOptions, huh.NewOption(label, d.ID()))
	}
	agentOptions = append(agentOptions, huh.NewOption("[←] Volver al menú principal", "back"))

	agentSelect := huh.NewSelect[string]().
		Title("¿Qué agente de terminal deseas utilizar?").
		Description("Selecciona el motor de IA que ejecutará la sesión").
		Options(agentOptions...).
		Value(&selectedAgent).
		Validate(func(val string) error {
			if val == "back" {
				return nil
			}
			d, err := session.GetDriver(val)
			if err != nil {
				return err
			}
			if !d.IsAvailable() {
				return fmt.Errorf("El agente '%s' no está instalado en tu sistema.\nInstalación: %s", d.DisplayName(), d.InstallHint())
			}
			return nil
		})

	agentForm := huh.NewForm(
		huh.NewGroup(agentSelect),
	).WithTheme(CustomHuhTheme())
	agentForm = c.prepareForm(agentForm)

	if err := agentForm.Run(); err != nil || selectedAgent == "back" {
		return nil
	}

	selectedDriver, err := session.GetDriver(selectedAgent)
	if err != nil {
		return err
	}

	var selectedPerm string = string(session.PermissionSupervised)

	permOptions := []huh.Option[string]{
		huh.NewOption(
			fmt.Sprintf("%s  Solo lectura       — Ejecuta MCPs/consultas, sin modificar el proyecto", icons.Shield),
			string(session.PermissionReadOnly),
		),
		huh.NewOption(
			fmt.Sprintf("%s  Con autorización   — Solicita confirmación [y/N] en cada acción", icons.Robot),
			string(session.PermissionSupervised),
		),
		huh.NewOption(
			fmt.Sprintf("%s  Autónomo           — Auto-aprueba ejecuciones dentro del workspace actual", icons.Bolt),
			string(session.PermissionAutonomous),
		),
		huh.NewOption(
			fmt.Sprintf("%s  Volver al menú principal", icons.Back),
			"back",
		),
	}

	permForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("%s Selecciona el nivel de permiso para %s:", icons.Key, selectedDriver.DisplayName())).
				Description("Define las reglas de ejecución del agente en tu entorno").
				Options(permOptions...).
				Value(&selectedPerm),
		),
	).WithTheme(CustomHuhTheme())
	permForm = c.prepareForm(permForm)

	if err := permForm.Run(); err != nil || selectedPerm == "back" {
		return nil
	}

	cwd, _ := os.Getwd()
	svc := c.getService(cwd)
	toolingSvc := c.getToolingService(cwd)

	var selectedToolings []string

	presets, _ := toolingSvc.ListPresets(context.Background())
	if len(presets) > 0 {
		var selectedChoice string = "__none__"
		var choiceOptions []huh.Option[string]

		for _, p := range presets {
			desc := p.Description
			if desc != "" {
				desc = " — " + desc
			}
			label := fmt.Sprintf("%s%s (%d toolkits)", p.Name, desc, len(p.Toolkits))
			choiceOptions = append(choiceOptions, huh.NewOption(label, p.Name))
		}

		choiceOptions = append(choiceOptions,
			huh.NewOption(fmt.Sprintf("%s  Sin perfil (sesión base sin toolkits adicionales)", icons.Shield), "__none__"),
			huh.NewOption(fmt.Sprintf("%s  Personalizado (seleccionar toolkits individuales...)", icons.Box), "__custom__"),
			huh.NewOption(fmt.Sprintf("%s  Inicializar nuevo toolkit (Scaffold)...", icons.Sparkle), "__scaffold__"),
		)

		profileForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("%s Selecciona el perfil agéntico para la sesión:", icons.Box)).
					Description("Carga automáticamente los toolkits y directivas correspondientes").
					Options(choiceOptions...).
					Value(&selectedChoice),
			),
		).WithTheme(CustomHuhTheme())
		profileForm = c.prepareForm(profileForm)

		if err := profileForm.Run(); err != nil {
			return nil
		}

		switch selectedChoice {
		case "__none__":
			selectedToolings = nil
		case "__custom__":
			var customToolings []string
			var customOptions []huh.Option[string]
			if toolkits, err := toolingSvc.ListToolkits(context.Background()); err == nil {
				for _, tk := range toolkits {
					desc := tk.Description
					if desc != "" {
						desc = " — " + desc
					}
					label := fmt.Sprintf("%s (%s, %d skills)%s", tk.ID, tk.Scope, len(tk.SkillPaths), desc)
					customOptions = append(customOptions, huh.NewOption(label, tk.ID))
				}
			}
			if len(customOptions) > 0 {
				customForm := huh.NewForm(
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title(fmt.Sprintf("%s Selecciona los toolkits individuales a activar:", icons.Box)).
							Description("Espacio para seleccionar/deseleccionar. Enter para continuar.").
							Options(customOptions...).
							Value(&customToolings),
					),
				).WithTheme(CustomHuhTheme())
				customForm = c.prepareForm(customForm)
				if err := customForm.Run(); err == nil {
					selectedToolings = customToolings
				}
			}
		case "__scaffold__":
			var (
				newTKID   string
				newTKDesc string
				newTKGlob bool
			)
			scaffoldForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Identificador del nuevo toolkit:").
						Description("Nombre único en kebab-case (ej. backend-go, devops-aws)").
						Value(&newTKID).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("el ID no puede estar vacío")
							}
							return nil
						}),
					huh.NewInput().
						Title("Descripción del toolkit:").
						Description("Propósito o directivas principales").
						Value(&newTKDesc),
					huh.NewConfirm().
						Title("¿Crear en ámbito global (~/.config/gz-ia/tooling) en lugar del proyecto (.harness)?").
						Value(&newTKGlob),
				),
			).WithTheme(CustomHuhTheme())
			scaffoldForm = c.prepareForm(scaffoldForm)

			if err := scaffoldForm.Run(); err == nil && strings.TrimSpace(newTKID) != "" {
				cleanID := strings.TrimSpace(newTKID)
				created, err := toolingSvc.CreateToolkit(context.Background(), tooling.CreateToolkitRequest{
					ID:          cleanID,
					Description: newTKDesc,
					Global:      newTKGlob,
				})
				if err == nil && created != nil {
					fmt.Fprintf(c.getOut(), "%s Toolkit '%s' creado exitosamente y seleccionado para la sesión.\n", icons.Check, cleanID)
					selectedToolings = append(selectedToolings, cleanID)
				} else {
					fmt.Fprintf(c.getOut(), "%s Error creando toolkit: %v\n", icons.Cross, err)
				}
			}
		default:
			selectedToolings = []string{selectedChoice}
		}
	} else {
		var toolkitOptions []huh.Option[string]
		if toolkits, err := toolingSvc.ListToolkits(context.Background()); err == nil {
			for _, tk := range toolkits {
				desc := tk.Description
				if desc != "" {
					desc = " — " + desc
				}
				label := fmt.Sprintf("%s (%s, %d skills)%s", tk.ID, tk.Scope, len(tk.SkillPaths), desc)
				toolkitOptions = append(toolkitOptions, huh.NewOption(label, tk.ID))
			}
		}

		toolkitOptions = append(toolkitOptions, huh.NewOption(
			fmt.Sprintf("%s [+] Inicializar nuevo toolkit (Scaffold)...", icons.Sparkle),
			"__scaffold__",
		))

		toolkitForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(fmt.Sprintf("%s Toolkits a activar (Opcional):", icons.Box)).
					Description("Espacio para seleccionar/deseleccionar. Enter para continuar.").
					Options(toolkitOptions...).
					Value(&selectedToolings),
			),
		).WithTheme(CustomHuhTheme())
		toolkitForm = c.prepareForm(toolkitForm)

		if err := toolkitForm.Run(); err != nil {
			return nil
		}

		var finalToolings []string
		hasScaffold := false
		for _, item := range selectedToolings {
			if item == "__scaffold__" {
				hasScaffold = true
			} else {
				finalToolings = append(finalToolings, item)
			}
		}

		if hasScaffold {
			var (
				newTKID   string
				newTKDesc string
				newTKGlob bool
			)
			scaffoldForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Identificador del nuevo toolkit:").
						Description("Nombre único en kebab-case (ej. backend-go, devops-aws)").
						Value(&newTKID).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("el ID no puede estar vacío")
							}
							return nil
						}),
					huh.NewInput().
						Title("Descripción del toolkit:").
						Description("Propósito o directivas principales").
						Value(&newTKDesc),
					huh.NewConfirm().
						Title("¿Crear en ámbito global (~/.config/gz-ia/tooling) en lugar del proyecto (.harness)?").
						Value(&newTKGlob),
				),
			).WithTheme(CustomHuhTheme())
			scaffoldForm = c.prepareForm(scaffoldForm)

			if err := scaffoldForm.Run(); err == nil && strings.TrimSpace(newTKID) != "" {
				cleanID := strings.TrimSpace(newTKID)
				created, err := toolingSvc.CreateToolkit(context.Background(), tooling.CreateToolkitRequest{
					ID:          cleanID,
					Description: newTKDesc,
					Global:      newTKGlob,
				})
				if err == nil && created != nil {
					fmt.Fprintf(c.getOut(), "%s Toolkit '%s' creado exitosamente y seleccionado para la sesión.\n", icons.Check, cleanID)
					finalToolings = append(finalToolings, cleanID)
				} else {
					fmt.Fprintf(c.getOut(), "%s Error creando toolkit: %v\n", icons.Cross, err)
				}
			}
		}

		selectedToolings = finalToolings
	}

	req := session.StartChatRequest{
		Provider:        selectedDriver.ID(),
		WorkingDir:      cwd,
		PermissionLevel: session.PermissionLevel(selectedPerm),
		BinaryPath:      selectedDriver.BinaryName(),
		Profiles:        selectedToolings,
		OnLaunch: func(id string, isIsolated bool) {
			var permLabel string
			switch session.PermissionLevel(selectedPerm) {
			case session.PermissionReadOnly:
				permLabel = "Solo lectura (--mode plan)"
			case session.PermissionAutonomous:
				permLabel = "Autónomo (--dangerously-skip-permissions)"
			default:
				permLabel = "Con autorización (interactivo [y/N])"
			}

			modeDesc := "Directorio directo (sin Git)"
			if isIsolated {
				modeDesc = fmt.Sprintf("Worktree aislado (.harness/worktrees/%s)", id)
			}

			toolingDesc := ""
			if len(selectedToolings) > 0 {
				toolingDesc = fmt.Sprintf(" | Toolkits/Presets: %s", strings.Join(selectedToolings, ", "))
			}

			launchHeader := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).
				Render(fmt.Sprintf("\n%s Iniciando %s [%s] en [%s] | Modo: %s | Workspace: %s%s...\n", icons.Rocket, selectedDriver.DisplayName(), id, cwd, permLabel, modeDesc, toolingDesc))
			fmt.Fprintln(c.getOut(), launchHeader)
		},
	}

	if err := svc.StartChat(context.Background(), req); err != nil {
		fmt.Fprintf(c.getOut(), "%s Error al iniciar %s: %v\n\n", icons.Cross, selectedDriver.DisplayName(), err)
		return err
	}

	return nil
}

// handleStartChat mantiene retrocompatibilidad con invocaciones previas.
func (c *Client) handleStartChat(icons IconSet) error {
	return c.handleNewChat(icons)
}

// handleSessions permite ver las sesiones registradas y finalizar sesiones activas (kill).
func (c *Client) handleSessions(icons IconSet) error {
	cwd, _ := os.Getwd()
	svc := c.getService(cwd)
	sessions, err := svc.List(context.Background())
	if err != nil {
		fmt.Printf("%s Error al listar sesiones: %v\n", icons.Cross, err)
		return err
	}

	if len(sessions) == 0 {
		fmt.Println(lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("\n%s No hay sesiones registradas en este proyecto.\n", icons.Sparkle)))
		return nil
	}

	options := make([]huh.Option[string], 0, len(sessions)+1)
	for _, s := range sessions {
		durationStr := "-"
		if s.DurationMs > 0 {
			durationStr = (time.Duration(s.DurationMs) * time.Millisecond).Round(time.Second).String()
		} else if s.Status == session.StatusRunning {
			durationStr = time.Since(s.StartedAt).Round(time.Second).String()
		}

		pidStr := "-"
		if s.PID > 0 {
			pidStr = fmt.Sprintf("PID:%d", s.PID)
		}

		modeTag := "Directo"
		if s.IsIsolated {
			modeTag = "Worktree"
		}

		label := fmt.Sprintf("[%s] %-9s | %-8s | %-8s | Permiso: %-10s | %s", s.ID, s.Status, modeTag, pidStr, s.PermissionLevel, durationStr)
		options = append(options, huh.NewOption(label, s.ID))
	}
	options = append(options, huh.NewOption(fmt.Sprintf("%s Volver al menú principal", icons.Back), "back"))

	var selectedID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("%s Sesiones Agénticas en el Proyecto:", icons.Worktree)).
				Description("Selecciona una sesión para ver detalles o finalizarla:").
				Options(options...).
				Value(&selectedID),
		),
	).WithTheme(CustomHuhTheme())

	form = c.prepareForm(form)
	if err := form.Run(); err != nil || selectedID == "back" {
		return nil
	}

	rec, err := svc.GetRecord(context.Background(), selectedID)
	if err != nil {
		fmt.Fprintf(c.getOut(), "%s Error: %v\n", icons.Cross, err)
		return err
	}

	return c.handleSessionDetail(rec, svc, icons)
}

// prepareForm configura I/O accesible en formularios Huh cuando se utilizan lectores/escritores personalizados.
func (c *Client) prepareForm(form *huh.Form) *huh.Form {
	if c.in != nil {
		form = form.WithInput(c.in).WithAccessible(true)
	}
	if c.out != nil {
		form = form.WithOutput(c.out)
	}
	return form
}

// renderWorktreeDiffViewer formatea el diff del worktree en un visor estilizado con Lipgloss.
func renderWorktreeDiffViewer(sessionID string, diff string, icons IconSet) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite).
		Background(ColorPrimary).
		Padding(0, 1)

	header := headerStyle.Render(fmt.Sprintf("%s Worktree Diff Viewer — Sesión: %s", icons.Search, sessionID))

	lines := strings.Split(diff, "\n")
	var formattedLines []string

	addStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	delStyle := lipgloss.NewStyle().Foreground(ColorDanger)
	hunkStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			formattedLines = append(formattedLines, addStyle.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			formattedLines = append(formattedLines, delStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			formattedLines = append(formattedLines, hunkStyle.Render(line))
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			formattedLines = append(formattedLines, metaStyle.Render(line))
		default:
			formattedLines = append(formattedLines, line)
		}
	}

	content := strings.Join(formattedLines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		MarginTop(1)

	return header + "\n" + boxStyle.Render(content)
}

// handleSessionDetail muestra los detalles de una sesión y las acciones disponibles (kill, resume, read, get, delete).
func (c *Client) handleSessionDetail(rec *session.SessionRecord, svc session.Service, icons IconSet) error {
	out := c.getOut()

	fmt.Fprintf(out, "\n%s Sesión: %s\n", icons.Terminal, rec.ID)
	fmt.Fprintf(out, "  Estado:     %s\n", rec.Status)
	fmt.Fprintf(out, "  PID:        %d\n", rec.PID)
	fmt.Fprintf(out, "  Permiso:    %s\n", rec.PermissionLevel)
	fmt.Fprintf(out, "  Directorio: %s\n", rec.WorkingDir)
	if rec.IsIsolated {
		fmt.Fprintf(out, "  Workspace:  Worktree aislado (.harness/worktrees/%s)\n", rec.ID)
		if rec.BranchName != "" {
			fmt.Fprintf(out, "  Rama:       %s\n", rec.BranchName)
		}
	} else {
		fmt.Fprintf(out, "  Workspace:  Directorio directo (sin Git)\n")
	}
	fmt.Fprintf(out, "  Inicio:     %s\n\n", rec.StartedAt.Format("2006-01-02 15:04:05"))

	var action string
	var actionOptions []huh.Option[string]

	if rec.Status == session.StatusRunning {
		actionOptions = []huh.Option[string]{
			huh.NewOption(fmt.Sprintf("%s Finalizar proceso (kill)", icons.Cross), "kill"),
			huh.NewOption(fmt.Sprintf("%s Inspeccionar worktree (read)", icons.Search), "read"),
			huh.NewOption(fmt.Sprintf("%s Traer cambios al workspace (get)", icons.Check), "get"),
			huh.NewOption(fmt.Sprintf("%s Eliminar registro de sesión", icons.Warning), "delete"),
			huh.NewOption(fmt.Sprintf("%s Volver", icons.Back), "back"),
		}
	} else {
		actionOptions = []huh.Option[string]{
			huh.NewOption(fmt.Sprintf("%s Revivir / Reanudar sesión (agy --continue)", icons.Sync), "resume"),
			huh.NewOption(fmt.Sprintf("%s Inspeccionar worktree (read)", icons.Search), "read"),
			huh.NewOption(fmt.Sprintf("%s Traer cambios al workspace (get)", icons.Check), "get"),
			huh.NewOption(fmt.Sprintf("%s Eliminar registro de sesión", icons.Cross), "delete"),
			huh.NewOption(fmt.Sprintf("%s Volver", icons.Back), "back"),
		}
	}

	actionForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Acciones disponibles:").
				Options(actionOptions...).
				Value(&action),
		),
	).WithTheme(CustomHuhTheme())
	actionForm = c.prepareForm(actionForm)

	if err := actionForm.Run(); err != nil || action == "back" {
		return nil
	}

	return c.executeSessionAction(action, rec, svc, icons)
}

// executeSessionAction ejecuta la acción seleccionada sobre una sesión.
func (c *Client) executeSessionAction(action string, rec *session.SessionRecord, svc session.Service, icons IconSet) error {
	out := c.getOut()
	in := c.getIn()

	switch action {
	case "kill":
		if err := svc.Kill(context.Background(), rec.ID); err != nil {
			fmt.Fprintf(out, "%s Error al terminar sesión: %v\n\n", icons.Cross, err)
		} else {
			fmt.Fprintf(out, "%s Sesión '%s' (PID %d) terminada.\n\n", icons.Check, rec.ID, rec.PID)
		}
	case "resume":
		launchHeader := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).
			Render(fmt.Sprintf("\n%s Reviviendo sesión [%s] en [%s]...\n", icons.Sync, rec.ID, rec.WorkingDir))
		fmt.Fprintln(out, launchHeader)

		if err := svc.Resume(context.Background(), rec.ID); err != nil {
			fmt.Fprintf(out, "%s Error al reanudar sesión: %v\n\n", icons.Cross, err)
			return err
		}
	case "read":
		diffOutput, err := svc.Read(context.Background(), rec.ID, false)
		if err != nil {
			fmt.Fprintf(out, "%s Error al inspeccionar worktree: %v\n\n", icons.Cross, err)
		} else if strings.TrimSpace(diffOutput) == "" {
			fmt.Fprintf(out, "%s No hay cambios registrados en el worktree de la sesión '%s'.\n\n", icons.Sparkle, rec.ID)
		} else {
			viewer := renderWorktreeDiffViewer(rec.ID, diffOutput, icons)
			fmt.Fprintln(out, viewer)
			fmt.Fprint(out, lipgloss.NewStyle().Foreground(ColorMuted).Render("\nPresiona Enter para continuar..."))
			_, _ = bufio.NewReader(in).ReadString('\n')
		}
	case "get":
		var confirm bool
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("¿Traer e integrar los cambios de la sesión '%s' al workspace activo?", rec.ID)).
					Description("Esta operación fusionará las modificaciones del worktree en la rama actual.").
					Affirmative(fmt.Sprintf("%s Integrar (get)", icons.Check)).
					Negative(fmt.Sprintf("%s Cancelar", icons.Cross)).
					Value(&confirm),
			),
		).WithTheme(CustomHuhTheme())
		confirmForm = c.prepareForm(confirmForm)

		if err := confirmForm.Run(); err == nil && confirm {
			res, err := svc.Get(context.Background(), rec.ID)
			if err != nil {
				fmt.Fprintf(out, "%s Error al traer cambios del worktree: %v\n\n", icons.Cross, err)
			} else if res.AlreadyUpToDate {
				fmt.Fprintf(out, "%s La rama ya está actualizada. No hay cambios pendientes.\n\n", icons.Sparkle)
			} else {
				successMsg := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).
					Render(fmt.Sprintf("%s Cambios del worktree de la sesión '%s' traídos al workspace activo (unstaged).\n", icons.Check, rec.ID))
				fmt.Fprintln(out, successMsg)
				if len(res.FilesIntegrated) > 0 {
					fmt.Fprintln(out, lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render("Archivos traídos (unstaged):"))
					for _, f := range res.FilesIntegrated {
						fmt.Fprintf(out, "  • %s\n", f)
					}
					fmt.Fprintln(out)
				}
			}
		}
	case "delete":
		var confirm bool
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("¿Eliminar la sesión '%s' de .harness/sessions/?", rec.ID)).
					Affirmative(fmt.Sprintf("%s Eliminar", icons.Check)).
					Negative(fmt.Sprintf("%s Cancelar", icons.Cross)).
					Value(&confirm),
			),
		).WithTheme(CustomHuhTheme())
		confirmForm = c.prepareForm(confirmForm)

		if err := confirmForm.Run(); err == nil && confirm {
			if err := svc.Delete(context.Background(), rec.ID); err != nil {
				fmt.Fprintf(out, "%s Error al eliminar sesión: %v\n\n", icons.Cross, err)
			} else {
				fmt.Fprintf(out, "%s Sesión '%s' eliminada correctamente.\n\n", icons.Check, rec.ID)
			}
		}
	}

	return nil
}

// handleUpdate comprueba si hay nuevas versiones disponibles y gestiona la actualización interactiva de gz-ia.
func (c *Client) handleUpdate(icons IconSet) error {
	ctx := context.Background()
	upd := c.getUpdater()
	out := c.getOut()
	in := c.getIn()

	defer func() {
		fmt.Fprint(out, lipgloss.NewStyle().Foreground(ColorMuted).Render("\nPresiona Enter para volver al menú principal..."))
		_, _ = bufio.NewReader(in).ReadString('\n')
	}()

	fmt.Fprintln(out, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).
		Render(fmt.Sprintf("\n%s Comprobando actualizaciones de gz-ia...", icons.Sync)))

	info, err := upd.CheckLatest(ctx)
	if err != nil {
		fmt.Fprintln(out, StyleDanger.Render(fmt.Sprintf("\n%s Error al verificar actualizaciones: %v\n", icons.Cross, err)))
		return err
	}

	if !info.IsNewer {
		currVer := updater.NormalizeVersion(info.CurrentVer)
		if currVer == "" {
			currVer = info.CurrentVer
		}
		msg := StyleSuccess.Render(fmt.Sprintf("\n%s gz-ia ya se encuentra en la versión más reciente (v%s).\n", icons.Check, currVer))
		fmt.Fprintln(out, msg)
		return nil
	}

	curV := updater.NormalizeVersion(info.CurrentVer)
	if curV == "" {
		curV = info.CurrentVer
	}
	newV := updater.NormalizeVersion(info.Version)
	if newV == "" {
		newV = info.Version
	}

	var confirm bool
	confirmTitle := fmt.Sprintf("Nueva versión disponible (v%s -> v%s). ¿Deseas descargar e instalar la actualización ahora?", curV, newV)

	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(confirmTitle).
				Affirmative(fmt.Sprintf("%s Actualizar", icons.Check)).
				Negative(fmt.Sprintf("%s Cancelar", icons.Cross)).
				Value(&confirm),
		),
	).WithTheme(CustomHuhTheme())

	if c.in != nil {
		confirmForm = confirmForm.WithInput(c.in).WithAccessible(true)
	}
	if c.out != nil {
		confirmForm = confirmForm.WithOutput(c.out)
	}

	if err := confirmForm.Run(); err != nil || !confirm {
		fmt.Fprintln(out, lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("\n%s Actualización cancelada.\n", icons.Exit)))
		return nil
	}

	fmt.Fprintln(out, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).
		Render(fmt.Sprintf("\n%s Descargando e instalando gz-ia v%s...", icons.Sync, newV)))

	res, err := upd.Update(ctx, "", "")
	if err != nil {
		fmt.Fprintln(out, StyleDanger.Render(fmt.Sprintf("\n%s Error al actualizar gz-ia: %v\n", icons.Cross, err)))
		return err
	}

	resVer := updater.NormalizeVersion(res.Version)
	if resVer == "" {
		resVer = res.Version
	}
	successMsg := StyleSuccess.Render(fmt.Sprintf("\n%s gz-ia actualizado exitosamente a la versión v%s\n", icons.Check, resVer))
	fmt.Fprintln(out, successMsg)

	return nil
}

func (c *Client) resolveWorkDir() string {
	if c.workspace != nil {
		return c.workspace.ResolveProjectRoot(context.Background(), "")
	}
	cwd, _ := os.Getwd()
	return cwd
}

// handleVault permite gestionar las variables de entorno y secretos del vault de forma segura e interactiva.
func (c *Client) handleVault(icons IconSet) error {
	workDir := c.resolveWorkDir()
	svc := c.getVault(workDir)
	ctx := context.Background()

	for {
		var action string
		menuTitle := fmt.Sprintf("%s  Vault de Secretos y Variables de Entorno", icons.Lock)
		menuDesc := fmt.Sprintf("%s Almacén seguro (.harness/vault.json, permisos 0600):", icons.Arrow)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(menuTitle).
					Description(menuDesc).
					Options(
						huh.NewOption(fmt.Sprintf("%s  Ver variables y recomendaciones (ofuscadas)", icons.Sparkle), "list"),
						huh.NewOption(fmt.Sprintf("%s  Configurar / Actualizar variable con valor secreto", icons.Bolt), "set"),
						huh.NewOption(fmt.Sprintf("%s  Eliminar variable del Vault", icons.Cross), "delete"),
						huh.NewOption("[←] Volver al menú principal", "back"),
					).
					Value(&action),
			),
		).WithTheme(CustomHuhTheme())

		if c.in != nil {
			form = form.WithInput(c.in).WithAccessible(true)
		}
		if c.out != nil {
			form = form.WithOutput(c.out)
		}

		if err := form.Run(); err != nil || action == "back" {
			return nil
		}

		switch action {
		case "list":
			statuses, err := svc.ListStatus(ctx, nil, "")
			if err != nil {
				fmt.Fprintln(c.getOut(), lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Error: %v", err)))
				continue
			}

			fmt.Fprintln(c.getOut())
			header := fmt.Sprintf("%-24s  %-12s  %-24s  %-20s", "VARIABLE", "ESTADO", "VALOR ENMASCARADO", "RECOMENDADA PARA")
			fmt.Fprintln(c.getOut(), lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(header))
			fmt.Fprintln(c.getOut(), lipgloss.NewStyle().Foreground(ColorSubtle).Render("─────────────────────────────────────────────────────────────────────────────────────────────"))

			for _, s := range statuses {
				statusBadge := lipgloss.NewStyle().Foreground(ColorMuted).Render("Faltante")
				if s.InVault {
					statusBadge = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("Vault [✓]")
				} else if s.InSystem {
					statusBadge = lipgloss.NewStyle().Foreground(ColorSecondary).Render("Sistema [$]")
				}

				masked := s.MaskedValue
				if masked == "" {
					masked = lipgloss.NewStyle().Foreground(ColorMuted).Render("(no configurada)")
				}

				rec := strings.Join(s.RecommendedFor, ", ")
				if rec == "" {
					rec = "-"
				}
				if len(rec) > 20 {
					rec = rec[:17] + "..."
				}

				line := fmt.Sprintf("%-24s  %-12s  %-24s  %-20s", s.Key, statusBadge, masked, rec)
				fmt.Fprintln(c.getOut(), line)
			}
			fmt.Fprintln(c.getOut())

		case "set":
			var key string
			var val string

			keyForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Nombre de la variable de entorno:").
						Description("Ejemplo: ANTHROPIC_API_KEY, GITHUB_TOKEN, DATABASE_URL").
						Value(&key),
				),
			).WithTheme(CustomHuhTheme())
			if c.in != nil {
				keyForm = keyForm.WithInput(c.in).WithAccessible(true)
			}
			if c.out != nil {
				keyForm = keyForm.WithOutput(c.out)
			}
			if err := keyForm.Run(); err != nil || strings.TrimSpace(key) == "" {
				continue
			}

			cleanKey := strings.TrimSpace(key)

			valForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("Valor secreto para '%s':", cleanKey)).
						Description("La entrada se oculta en pantalla por seguridad").
						EchoMode(huh.EchoModePassword).
						Value(&val),
				),
			).WithTheme(CustomHuhTheme())
			if c.in != nil {
				valForm = valForm.WithInput(c.in).WithAccessible(true)
			}
			if c.out != nil {
				valForm = valForm.WithOutput(c.out)
			}
			if err := valForm.Run(); err != nil {
				continue
			}

			if err := svc.Set(ctx, cleanKey, val); err != nil {
				fmt.Fprintln(c.getOut(), lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Error guardando variable: %v", err)))
			} else {
				msg := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).
					Render(fmt.Sprintf("\n%s Variable '%s' guardada de forma segura en el Vault (%s)\n", icons.Check, cleanKey, svc.VaultPath()))
				fmt.Fprintln(c.getOut(), msg)
			}

		case "delete":
			var keyToDelete string
			delForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Nombre de la variable a eliminar del Vault:").
						Value(&keyToDelete),
				),
			).WithTheme(CustomHuhTheme())
			if c.in != nil {
				delForm = delForm.WithInput(c.in).WithAccessible(true)
			}
			if c.out != nil {
				delForm = delForm.WithOutput(c.out)
			}
			if err := delForm.Run(); err != nil || strings.TrimSpace(keyToDelete) == "" {
				continue
			}

			cleanKey := strings.TrimSpace(keyToDelete)
			if err := svc.Delete(ctx, cleanKey); err != nil {
				fmt.Fprintln(c.getOut(), lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Error: %v", err)))
			} else {
				msg := lipgloss.NewStyle().Foreground(ColorSuccess).
					Render(fmt.Sprintf("\n%s Variable '%s' eliminada del Vault.\n", icons.Check, cleanKey))
				fmt.Fprintln(c.getOut(), msg)
			}
		}
	}
}
