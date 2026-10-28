# Decisiones y estado de implementación

El usuario aprobó diseño y ejecución. El código está en el repositorio local `sor-hosting`, rama `feat/initial-platform`. No se ha publicado una imagen ni ejecutado un despliegue sobre el VPS.

| Decisión | Motivo | Implicación |
|---|---|---|
| Proyecto nuevo en su propia carpeta y rama | No había aplicación previa que aislar | Los repositorios vecinos no forman parte de los cambios |
| Verificar Go posteriormente en un entorno habilitado | No hay Go/sqlc y la terminal no resuelve DNS; Docker y Chromium también tienen restricciones | La compilación y los tests Go y las integraciones deben pasar antes de producción; no se declaran verificados |
| Generar paquetes sqlc durante bootstrap/build | Mantener SQL como fuente y evitar simular archivos generados | La primera compilación necesita sqlc y resolver dependencias; `go.sum` se genera al resolver los módulos |
| Compose para el ciclo WordPress, SDK para estado y operaciones Docker | Reutilizar la orquestación declarativa con secretos montados y argumentos fijos | El VPS requiere Docker CLI y Compose v2 además del Engine |
| Estado `pending_ssl` | Separar DNS pendiente de instalación terminada con certificado pendiente | Los clientes de la API deben reconocer este estado adicional |
| Restaurar sitios con escritores extra después de migrarlos a la plantilla soportada | No se pueden recrear cargas adicionales a partir de sus IDs sin recetas declaradas | Esos sitios se pueden respaldar, pero necesitan migración antes de restaurar mediante staging |

La fuente Fira Sans se sirve localmente con licencia OFL incluida. Fira Code utiliza fallback monospace del sistema mientras no se añada su archivo local. Esta es una diferencia visual menor documentada, sin degradados en la interfaz.

El agente nativo se instala y actualiza por el operador, sin trabajos activos. CI/CD construye los componentes y publica/despliega la imagen del panel; no sustituye un agente privilegiado durante una restauración.

## Evidencia final de esta sesión

- Cliente HTTP: **4 casos pasan**, incluidos sesión invalidada, respuesta privada tardía, CSRF y error de red en español.
- TypeScript y Vite: **build de producción correcto**.
- SQLite: **9 comprobaciones pasan**, incluidas unicidad, claves foráneas, sesiones en cascada, idempotencia, estados y preparación de todas las consultas de backend/agente.
- Compose: **configuración válida** con variables de prueba sin secretos reales.
- Scripts: **sintaxis Bash válida**, instalador en dry-run correcto.
- Contratos y configuración: JSON/YAML y referencias OpenAPI verificados.
- Go: `go: command not found`. No se han ejecutado los tests Go ni generado los paquetes sqlc aquí.
- Navegador: se intentó el E2E; Chromium abortó en Crashpad con `setsockopt: Operation not permitted` y `SIGTRAP`. No se han declarado capturas ni flujos reales de navegador verificados.
- Docker Engine/SSL/WordPress/backup/restore: requieren pruebas reales en host desechable; no se ha accedido al VPS durante esta implementación.

Se realizó revisión independiente de la lógica de runtime. Se corrigieron los permisos de secretos afectados por UMask, la persistencia de adopciones rechazadas, las asociaciones duplicadas, los reintentos tras perder un 202 y los trabajos bloqueados al sincronizar resultados. Se añadieron regresiones Go para ejecutarse con el toolchain completo. Las firmas de Moby utilizadas se contrastaron con su documentación oficial; esto no sustituye compilar ni probar.
