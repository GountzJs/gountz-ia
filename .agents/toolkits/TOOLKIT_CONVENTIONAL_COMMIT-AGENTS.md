# Directivas Maestras — toolkit-conventional-commit

Define las convenciones normativas, especificaciones tecnicas y procedimientos de control para la creacion de commits bajo el estandar Conventional Commits v1.0.0.

---

## 1. Filosofia Operativa

1. **Trazabilidad Semantica**:
   Todo mensaje de commit debe comunicar con precision la naturaleza del cambio (funcionalidad, correccion, documentacion, refactorizacion, mantenimiento) para facilitar la auditoria automatizada, la generacion de changelogs y el versionado semantico (SemVer).

2. **Atomicidad de Cambios**:
   Cada commit debe contener una unica unidad logica de cambio. Queda prohibido mezclar refactorizaciones con correcciones de errores o cambios de estilo en un mismo commit.

3. **Cero Ruido (Zero-Noise Policy)**:
   - Prohibido el uso de emojis, caracteres decorativos o frases genericas como "changes", "fix", "update".
   - Todo mensaje debe redactarse en imperativo presente (ej. "add feature", "fix null pointer", no "added" ni "fixing").
   - El encabezado debe escribirse en minusculas (excepto siglas de estandares o identificadores propios como CLI, POSIX, TTY) y sin punto final.

---

## 2. Estructura Formal del Commit

El commit debe cumplir la siguiente gramatica estricta:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Reglas Estructurales

1. **Encabezado (Header)**:
   - Longitud maxima de 72 caracteres.
   - `<type>`: Obligatorio. Debe pertenecer a la lista de tipos canónicos.
   - `[optional scope]`: Opcional, pero fuertemente recomendado. Encerrado entre parentesis, en minusculas y kebab-case (ej. `cli`, `session`, `vault`, `tooling`, `docs`).
   - `: ` (dos puntos y espacio): Separador obligatorio.
   - `<description>`: Obligatorio. Resumen conciso del cambio en imperativo presente.

2. **Cuerpo (Body)**:
   - Opcional. Separado del encabezado por una linea en blanco.
   - Explica el *por que* y el *como* del cambio, no el *que* (el diff ya muestra el que).
   - Limite de 72 a 80 caracteres por linea.

3. **Pie (Footer)**:
   - Opcional. Separado del cuerpo por una linea en blanco.
   - Para cambios que rompen compatibilidad: `BREAKING CHANGE: <descripcion>` o un signo de exclamacion `!` antes de los dos puntos en el encabezado (`feat(api)!: remove deprecated endpoint`).
   - Para referencias a incidencias: `Closes #123`, `Fixes #456`.

---

## 3. Tipos Canonicos Autorizados

| Tipo | Proposito | Modifica SemVer |
| :--- | :--- | :--- |
| `feat` | Nueva funcionalidad o capacidad incorporada al sistema | MINOR (0.X.0) |
| `fix` | Correccion de un bug o comportamiento anomalo previo | PATCH (0.0.X) |
| `docs` | Modificaciones exclusivas en documentacion, guias o comentarios | Ninguno |
| `style` | Cambios de formato, espacios en blanco o punto y coma que no afectan logica | Ninguno |
| `refactor` | Modificacion de codigo que no corrige bugs ni anade funcionalidades | Ninguno |
| `perf` | Optimizacion de rendimiento o consumo computacional | PATCH (0.0.X) |
| `test` | Incorporacion o modificacion de suites de prueba | Ninguno |
| `build` | Modificaciones en sistema de compilacion, dependencias o Makefile | Ninguno |
| `ci` | Cambios en flujos de integracion continua (GitHub Actions, scripts CI) | Ninguno |
| `chore` | Tareas de mantenimiento, sincronizaciones o configuraciones auxiliares | Ninguno |
| `revert` | Reversion de un commit previo | Segun commit revertido |
