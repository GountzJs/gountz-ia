# Plugins y Service Container

El diseño modular de Orchy se apoya en dos pilares arquitectónicos: un **Contenedor IoC de Servicios** para inyección de dependencias desacoplada y un **Sistema de Plugins** con ciclo de vida gestionado.

---

## Contenedor de Servicios IoC (`ServiceContainer`)

El `core.ServiceContainer` (`packages/orchy/core/container.go`) permite registrar y resolver servicios de forma segura y tipada en tiempo de ejecución:

```go
// Registrar una instancia concreta (Singleton)
container.Register("database", myDatabaseService)

// Resolver un servicio registrado
if db, ok := container.Resolve("database").(MyDatabase); ok {
    db.Query(...)
}
```

### Ventajas
- **Desacoplamiento Total:** Los módulos no se conocen entre sí ni dependen de variables globales.
- **Testeabilidad Extrema:** En pruebas unitarias, cualquier dependencia puede ser reemplazada fácilmente por un mock.

---

## Interfaz y Ciclo de Vida del Plugin

Un plugin en Orchy es cualquier estructura que implemente la interfaz `plugins.Plugin` (`packages/orchy/plugins/plugin.go`):

```go
type Plugin interface {
    Name() string
    Version() string
    OnBoot(ctx *core.KernelContext) error
    OnShutdown(ctx *core.KernelContext) error
}
```

### Creación Rápida con `NewPlugin`

Orchy proporciona la función constructora funcional `orchy.NewPlugin` para crear plugins sin necesidad de declarar structs auxiliares:

```go
authPlugin := orchy.NewPlugin(
    "auth-service",
    "1.0.0",
    func(ctx *core.KernelContext) error {
        // Inicialización en Boot
        ctx.Services.Register("auth", NewAuthManager())
        return nil
    },
    func(ctx *core.KernelContext) error {
        // Limpieza en Shutdown
        return nil
    },
)

kernel.GetContext().Plugins.Register(authPlugin)
```

---

## Bus Tipado de Eventos (`EventBus`)

Para comunicación reactiva entre plugins y servicios sin acoplamiento directo, Orchy incluye un `events.EventBus` (`packages/orchy/events/bus.go`) de alto rendimiento con dos patrones:

### 1. Patrón Pub/Sub (Publicación y Suscripción)

Permite emitir eventos a múltiples suscriptores asíncronos o síncronos:

```go
// Suscribirse a un tópico
disposable := kernel.GetContext().Events.Subscribe("workspace.modified", func(payload any) {
    event := payload.(WorkspaceEvent)
    fmt.Printf("Archivo modificado: %s\n", event.Path)
})

// Desuscribirse cuando no sea necesario
defer disposable.Dispose()

// Publicar un evento
kernel.GetContext().Events.Publish("workspace.modified", WorkspaceEvent{Path: "main.go"})
```

### 2. Patrón Request/Response (RPC Interno)

Permite invocar servicios y recibir respuestas sin conocer la implementación concreta:

```go
// Registrar un manejador RPC
kernel.GetContext().Events.RegisterHandler("git.branch.current", func(ctx context.Context, req any) (any, error) {
    return "main", nil
})

// Ejecutar la petición
res, err := kernel.GetContext().Events.Request(ctx, "git.branch.current", nil)
```
