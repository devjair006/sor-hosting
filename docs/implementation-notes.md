# Decisiones y estado de implementación

El usuario aprobó diseño y ejecución. El código está en el repositorio local `sor-hosting`, rama `feat/initial-platform`. No se ha publicado una imagen ni ejecutado un despliegue sobre el VPS.

| Decisión | Motivo | Implicación |
|---|---|---|
| Proyecto nuevo en su propia carpeta y rama | No había aplicación previa que aislar | Los repositorios vecinos no forman parte de los cambios |
| Toolchain Go local verificado | Al habilitarse permisos y red, se descargó Go 1.26.10 y se verificó su SHA256 oficial | Bootstrap, compilación y tests Go ya se ejecutan; `.tools/go` no se versiona |
| Generar paquetes sqlc durante bootstrap/build | Mantener SQL como fuente y evitar simular archivos generados | La primera compilación necesita sqlc y resolver dependencias; `go.sum` está versionado para los tres módulos |
| Compose para el ciclo WordPress, SDK para estado y operaciones Docker | Reutilizar la orquestación declarativa con secretos montados y argumentos fijos | El VPS requiere Docker CLI y Compose v2 además del Engine |
| Estado `pending_ssl` | Separar DNS pendiente de instalación terminada con certificado pendiente | Los clientes de la API deben reconocer este estado adicional |
| Restaurar sitios con escritores extra después de migrarlos a la plantilla soportada | No se pueden recrear cargas adicionales a partir de sus IDs sin recetas declaradas | Esos sitios se pueden respaldar, pero necesitan migración antes de restaurar mediante staging |

La fuente Fira Sans se sirve localmente con licencia OFL incluida. Fira Code utiliza fallback monospace del sistema mientras no se añada su archivo local. Esta es una diferencia visual menor documentada, sin degradados en la interfaz.

El agente nativo se instala y actualiza por el operador, sin trabajos activos. CI/CD construye los componentes y publica/despliega la imagen del panel; no sustituye un agente privilegiado durante una restauración.

## Evidencia verificada

- Bootstrap/sqlc, tests Go, `go vet`, binarios Go sin CGO y build TypeScript/Vite: correctos.
- Cliente HTTP: cuatro casos; SQLite/esquemas: nueve casos correctos.
- Chromium: flujos con API controlada en cuatro tamaños y prueba adicional con Go/SQLite reales, incluida persistencia tras reiniciar.
- Docker real: instalación WordPress/MariaDB, operaciones autorizadas, backup de archivos y DB y restauración en staging con respaldo de seguridad; limpieza de los recursos de prueba confirmada.
- Imágenes Docker del panel y artefacto del agente construidas. Panel probado con UID 10001, read-only y sin capabilities, incluidos login de producción, CSRF y healthcheck.
- Compose, scripts e instalador dry-run: correctos.
- Nginx y Certbot del VPS, HTTPS público, Prometheus del VPS y el despliegue Azure quedan pendientes de validar en ese host. Las pruebas locales no han accedido al VPS.

Al resolver las dependencias reales se ajustaron los tipos `network.Port` y `netip.Addr` del SDK Moby y se eliminó un import sin uso. La integración detectó que sqlc 1.29 truncaba consultas SQLite posteriores a un literal Unicode; ahora la fase de recuperación se pasa como parámetro y un test de inventario/diario ejecuta las consultas generadas, no solo el SQL fuente. Se completó además el lockfile de npm con dependencias opcionales ausentes.

La revisión independiente anterior corrigió los permisos de secretos afectados por UMask, la persistencia de adopciones rechazadas, las asociaciones duplicadas, los reintentos tras perder un 202 y los trabajos bloqueados al sincronizar resultados. Sus regresiones Go ahora pasan con el toolchain completo.
