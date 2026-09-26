# Gestión Segura de Secretos y Variables de Entorno (Vault)

Gountz IA (`gz-ia`) incluye un sistema centralizado y seguro de almacenamiento de secretos y variables de entorno (`internal/features/vault`), diseñado para suministrar credenciales a los agentes de IA sin exponerlas en texto claro ni comitearlas accidentalmente en el historial de Git.

---

## 1. Filosofía de Seguridad y Almacenamiento

El Vault se gestiona mediante el archivo `.harness/vault.json`:
- **Permisos Estrictos POSIX `0600`:** Únicamente el usuario propietario del sistema operativo tiene permisos de lectura y escritura (`rw-------`). Cualquier intento de lectura o modificación por otros usuarios del sistema es bloqueado por el kernel Linux.
- **Escrituras Atómicas:** Toda modificación (`set`, `delete`) escribe primero en un archivo temporal (`.vault.json.tmp`) antes de invocar `os.Rename`, previniendo corrupción de datos ante interrupciones forzadas.
- **Aislamiento en `.gitignore`:** El directorio `.harness/` se encuentra excluido automáticamente del seguimiento de Git mediante `.git/info/exclude` o `.gitignore`, garantizando que ninguna API key viaje al repositorio remoto.

---

## 2. Comandos CLI: `gz-ia vault`

El comando `gz-ia vault` (o `gz-ia vault list` por defecto) administra el ciclo de vida de los secretos:

### Listar Variables y Estado de Configuración
```bash
gz-ia vault list [-d <directorio>]
```
**Salida de ejemplo:**
```text
🔒 Vault de Secretos y Variables de Entorno — Gountz IA
  Archivo: /home/usuario/proyecto/.harness/vault.json (Permisos 0600, ignorado por Git)

VARIABLE                  ESTADO        VALOR ENMASCARADO         RECOMENDADA PARA    
─────────────────────────────────────────────────────────────────────────────────────────────
ANTHROPIC_API_KEY         Vault [✓]     sk-ant-a************34    claude
OPENAI_API_KEY            Sistema [$]   sk-proj-************99    opencode
DATABASE_URL              Faltante      (no configurada)          data
```
- **Estados identificados:**
  - `Vault [✓]`: Almacenada y protegida en `.harness/vault.json`.
  - `Sistema [$]`: Detectada en las variables de entorno del sistema operativo (`$ENV`).
  - `Faltante`: Variable recomendada por un agente o perfil activo pero no encontrada.
- **Valores protegidos:** Todos los secretos se muestran ofuscados (`MaskSecret`), revelando solo el prefijo y sufijo mínimo necesario.

### Guardar o Actualizar una Variable
```bash
# Modo interactivo seguro (recomendado: entrada enmascarada sin rastro en el historial de shell)
gz-ia vault set ANTHROPIC_API_KEY

# Modo directo pasando el valor como argumento o mediante el flag -v
gz-ia vault set ANTHROPIC_API_KEY "sk-ant-api03-xxxx..."
# o
gz-ia vault set ANTHROPIC_API_KEY -v "sk-ant-api03-xxxx..."
```
> [!TIP] Prevención de fuga en `.bash_history`
> Si omites el valor al ejecutar `gz-ia vault set <CLAVE>`, el comando abre un formulario interactivo Huh con `EchoModePassword`. El texto introducido no se imprime en pantalla ni queda registrado en el historial de tu shell.

### Consultar el Estado y Origen de una Variable
```bash
# Inspección segura con valor enmascarado
gz-ia vault get ANTHROPIC_API_KEY

# Revelar el valor completo en texto plano (útil para tuberías y scripts)
gz-ia vault get ANTHROPIC_API_KEY --reveal
```

### Eliminar una Variable
```bash
gz-ia vault delete ANTHROPIC_API_KEY
# Aliases disponibles:
gz-ia vault rm ANTHROPIC_API_KEY
gz-ia vault remove ANTHROPIC_API_KEY
```

### Obtener la Ruta Física del Almacén
```bash
gz-ia vault path
```
Imprime la ruta absoluta a `.harness/vault.json`, permitiendo su referencia en scripts o diagnósticos.

---

## 3. Detección Proactiva al Iniciar Sesiones

Al iniciar una sesión de chat (`gz-ia chat` o desde la TUI), el arnés analiza automáticamente los requisitos del agente y de los perfiles agénticos activos:
1. **Validación de Proveedores:**
   - Si se utiliza `claude`, valida la presencia de `ANTHROPIC_API_KEY`.
   - Si se utiliza `opencode`, valida `OPENAI_API_KEY`.
2. **Validación de Perfiles:**
   - Si el perfil define variables en su campo `env`, verifica si existen en el sistema o en el vault.
3. **Alerta No Bloqueante:**
   Si detecta que una variable requerida no está configurada, despliega un aviso claro en pantalla antes de iniciar la sesión:
   ```text
   ⚠️  Aviso de Entorno: Se detectaron variables no configuradas en el entorno ni en el vault:
      • ANTHROPIC_API_KEY
      Puedes configurarlas en el vault seguro con: gz-ia vault set ANTHROPIC_API_KEY
   ```

---

## 4. Inyección Transparente en Procesos de IA

Cuando el agente (`agy`, `claude`, etc.) es lanzado por el ejecutor del sistema operativo (`OSRunner`):
- `gz-ia` fusiona las variables de entorno actuales del sistema con las almacenadas en el vault.
- Las variables del vault tienen precedencia segura y se inyectan en el entorno del proceso hijo sin alterar las variables globales del shell del usuario.
