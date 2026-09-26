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

### Listar Variables y Estado de Configuración
```bash
gz-ia vault list
```
**Salida de ejemplo:**
```text
VARIABLE             ESTADO               ORIGEN              VALOR               
ANTHROPIC_API_KEY    CONFIGURADA          entorno             sk-a****************
OPENAI_API_KEY       CONFIGURADA          vault               sk-p****************
NODE_ENV             NO CONFIGURADA       -                   -                   
```
- Muestra si la variable proviene del entorno (`os.Environ`) o del archivo del vault.
- Por defecto, los valores sensibles son enmascarados mostrando únicamente los primeros 4 y últimos 4 caracteres.

### Guardar o Actualizar una Variable
```bash
gz-ia vault set ANTHROPIC_API_KEY "sk-ant-api03-xxxx..."
```

### Consultar el Valor de una Variable
```bash
# Consulta segura (enmascarada)
gz-ia vault get ANTHROPIC_API_KEY

# Revelar el valor en texto claro (útil para scripts de shell)
gz-ia vault get ANTHROPIC_API_KEY --reveal
```

### Eliminar una Variable
```bash
gz-ia vault delete ANTHROPIC_API_KEY
```

### Obtener la Ruta Física del Almacén
```bash
gz-ia vault path
```

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
