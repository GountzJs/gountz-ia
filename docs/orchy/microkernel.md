# Microkernel Orchy: Conceptos y Filosofía

**Orchy** (`packages/orchy`) es el microkernel modular, desacoplado y extensible embebido dentro de `gz-ia`.

Inspirado en la filosofía Unix y los principios de microkernel del sistema operativo, Orchy proporciona un entorno de ejecución seguro donde herramientas, servicios y plugins operan con aislamiento, supervisión de salud y protocolos estrictos de comunicación.

---

## Filosofía: El "Kernel Honesto" (Honest Kernel)

En los sistemas tradicionales de agentes de IA, los errores de herramientas externas suelen ser silenciados, capturados genéricamente o falseados como respuestas vacías. Esto provoca alucinaciones en el modelo de lenguaje, que asume que una operación se ejecutó con éxito cuando en realidad falló silenciosamente.

Orchy se basa en el principio del **Kernel Honesto**:
- **Cero Falsos Positivos:** Si una herramienta falla, el kernel reporta el error exacto con su traza y estado de salud.
- **Transparencia Radical:** El agente recibe información fidedigna sobre la capacidad o degradación del sistema.
- **Aislamiento de Fallos:** El colapso de un plugin o herramienta externa nunca debe comprometer la estabilidad del kernel ni de otros servicios.

---

## Ciclo de Vida del Microkernel

El microkernel implementa una máquina de estados estricta para garantizar inicialización ordenada y limpieza segura de recursos:

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Booting: Kernel.Boot()
    Booting --> Running: Plugins OnBoot() completados
    Running --> ShuttingDown: Kernel.Shutdown() / Señal POSIX
    ShuttingDown --> Stopped: Plugins OnShutdown() completados
    Stopped --> [*]
```

### Estados del Kernel (`KernelState`)
1. **`KernelStateIdle`:** Estado inicial. Se registran plugins y servicios en el contenedor.
2. **`KernelStateBooting`:** El kernel ejecuta en orden de dependencia el método `OnBoot(ctx)` de cada plugin.
3. **`KernelStateRunning`:** Todos los servicios y herramientas están listos para recibir llamadas y procesar eventos.
4. **`KernelStateShuttingDown`:** Se rechazan nuevas invocaciones y se ejecuta `OnShutdown(ctx)` en orden inverso de registro.
5. **`KernelStateStopped`:** Recursos liberados y buses cerrados limpiamente.

---

## Contexto Unificado (`KernelContext`)

Cuando un plugin o herramienta se ejecuta, recibe un puntero a `core.KernelContext` (`packages/orchy/core/context.go`), el cual centraliza el acceso a todos los subsistemas:

```go
type KernelContext struct {
    Services *ServiceContainer // Contenedor IoC de inyección de dependencias
    Events   *EventBus         // Bus tipado de eventos Pub/Sub y Request/Response
    Tools    *ToolRegistry     // Registro de herramientas con Circuit Breaker
    Plugins  *PluginRegistry   // Catálogo de plugins cargados
    Ctx      context.Context   // Contexto raíz de cancelación de Go
}
```

---

## Circuit Breaker de 3 Estados para Herramientas

Cada herramienta registrada en el `ToolRegistry` es envuelta automáticamente por un proxy de supervisión (`tools.ToolProxy` (`packages/orchy/tools/proxy.go`)) que implementa un patrón **Circuit Breaker**:

```mermaid
stateDiagram-v2
    direction LR
    HEALTHY --> DEGRADED: Errores esporádicos o alta latencia
    DEGRADED --> HEALTHY: Invocaciones exitosas consecutivas
    DEGRADED --> DEAD: Fallos continuos > umbral
    DEAD --> DEGRADED: Health check / reset manual
```

### Los 3 Estados de Salud (`ToolStatus`)

1. **`ToolStatusHealthy` (`HEALTHY`):**
   - La herramienta opera con normalidad dentro de los límites esperados de latencia y tasa de error ($0\%$).
   - Las solicitudes del agente son despachadas de inmediato.
2. **`ToolStatusDegraded` (`DEGRADED`):**
   - La herramienta ha presentado fallos intermitentes o latencia elevada.
   - Las llamadas continúan permitiéndose, pero se emite una advertencia estructurada al agente y al logger para que considere estrategias alternativas.
3. **`ToolStatusDead` (`DEAD`):**
   - El circuito se abre tras exceder el umbral de fallos consecutivos.
   - Las solicitudes son rechazadas inmediatamente (*fail-fast*) sin esperar tiempos muertos de red o timeouts, notificando al agente que la herramienta no está disponible temporalmente.

---

## Ejemplo: Registro de una Herramienta con Orchy

```go
package main

import (
    "context"
    "gz-ia/packages/orchy"
)

func main() {
    kernel := orchy.NewKernel()

    // Registrar una herramienta segura en el kernel
    kernel.GetContext().Tools.Register(orchy.NewTool(
        "calcular_hash",
        "Calcula el hash SHA-256 de una cadena",
        orchy.ToolSchema{
            Type: "object",
            Properties: map[string]any{
                "input": map[string]string{"type": "string"},
            },
            Required: []string{"input"},
        },
        func(ctx context.Context, input any) (any, error) {
            // Lógica de ejecución
            return map[string]string{"hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}, nil
        },
    ))

    _ = kernel.Boot()
    defer kernel.Shutdown()
}
```
