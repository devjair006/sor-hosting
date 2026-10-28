# Verificación

## Comandos

`make check` genera sqlc, ejecuta tests Go de los tres módulos, go vet, tests del cliente, typecheck y build de producción. `make fmt` aplica gofmt. `python3 tools/check-schema.py` prueba el esquema y sus restricciones usando SQLite real. `npm run test:e2e --prefix frontend` verifica el build con Playwright y una API controlada, sin necesitar una VPS real.

## Verificado en desarrollo

Las restricciones iniciales de DNS, Docker y Chromium cambiaron al habilitarse el acceso del entorno. Se descargó Go 1.26.10 desde go.dev y se comprobó el SHA256 oficial antes de extraerlo en `.tools/go`. `make` y el bootstrap detectan ese toolchain local; también admiten Go instalado en el sistema.

- `./tools/bootstrap.sh`: sqlc 1.29.0, módulos Go y `npm ci` correctos. Los `go.sum` quedan versionados.
- `make check build`: generación sqlc, tests Go de los tres módulos, `go vet`, cuatro tests del cliente HTTP, TypeScript, Vite y ambos binarios con `CGO_ENABLED=0` correctos.
- `python3 tools/check-schema.py`: nueve casos SQLite correctos, incluida preparación de consultas.
- `npm run test:e2e --prefix frontend`: login, errores, navegación, operaciones, recursos protegidos y logout en 375/768/1024/1440 px, con API controlada.
- `npm run test:live --prefix frontend`: Chromium frente a la API Go y SQLite reales. Verifica login, cookies HttpOnly/SameSite, CSRF, dashboard sin integraciones, configuración, auditoría, persistencia de datos/sesión tras reiniciar y revocación al salir. Usa datos y credenciales temporales, sin mocks HTTP.
- Integración Docker real: instalación WordPress/MariaDB, puerto web loopback y DB sin publicar, IDs exactos, protección de contenedores, stop/start/restart, logs, contenido de archivos y DB, backup, modificación y restauración con staging, respaldo de seguridad y conservación del presupuesto. Los proyectos aleatorios se eliminan con sus volúmenes al terminar.
- Imágenes Docker `panel` y `agent-artifact`: construcción correcta. La imagen del panel arrancó con UID 10001, filesystem de solo lectura y sin capabilities; pasaron readiness, frontend estático, cookie de producción Secure/HttpOnly/SameSite, CSRF, configuración, logout y healthcheck.
- Compose, sintaxis Bash e instalador dry-run correctos.

La prueba Docker usa Nginx simulado sobre archivos temporales y un dominio `.invalid`. No ejecuta Nginx/Certbot del host ni prueba la emisión de certificados. La validación del proxy y sus rollbacks también tiene tests Go con ejecutores controlados.

## Repetir las integraciones

Después de `make build` y con Chromium instalado:

```bash
BROWSER_EXECUTABLE=/usr/bin/chromium npm run test:e2e --prefix frontend
BROWSER_EXECUTABLE=/usr/bin/chromium npm run test:live --prefix frontend
bash tools/test-docker.sh
```

Omite `BROWSER_EXECUTABLE` para usar Chromium instalado mediante Playwright. El script Docker requiere Engine/Compose y descarga las imágenes oficiales `wordpress:php8.3-apache`, `mariadb:11.4` y `wordpress:cli-php8.3`; resuelve sus digests antes de ejecutar. Puedes fijarlas mediante `SOR_TEST_WP_IMAGE`, `SOR_TEST_DB_IMAGE` y `SOR_TEST_CLI_IMAGE`. Nunca ejecuta una limpieza global de Docker. CI incluye las pruebas de navegador y esta integración antes de publicar.

Los tests del cliente usan `node:test` directamente. Las capturas de navegador se guardan en `frontend/test-results/`, excluido de Git.

## Prueba previa a producción

En un host Linux desechable con Nginx, Certbot de staging, Docker y Prometheus:

1. Instalar agente/panel, crear admin, comprobar login/logout/CSRF y acceso directo a rutas.
2. Probar 375/768/1024/1440 px, teclado, foco de diálogos, zoom 200 %, reduced-motion y contraste.
3. Crear WordPress con DNS correcto e incorrecto; comprobar readiness DB, puerto web loopback, DB sin publicar y reintento SSL.
4. Denegar acciones/logs de contenedores no autorizados y todos los recursos protegidos. Probar nombres parecidos e IDs reemplazados.
5. Forzar fallo de nginx -t y comprobar rollback; probar renovación selectiva, DNS mixto y Certbot no elegible.
6. Crear contenido y DB, respaldar, modificar, restaurar y verificar ambos. Probar checksum inválido, espacio insuficiente, symlinks/traversal y fallo del snapshot.
7. Matar API/agente durante una operación y durante mantenimiento/cutover. Confirmar recuperación y que no se repiten efectos inciertos.
8. Recrear contenedor de API y comprobar persistencia. Probar release con imagen no ready y rollback compatible.

Los escenarios de host, SSL, interrupciones y despliegue que aún no cubren las pruebas locales deben quedar registrados antes de habilitar operaciones sobre sitios reales. Los fixtures del navegador se limitan al código de pruebas y nunca se habilitan como modo de producción.

## Extensión Docker edge

`bash tools/test-docker-edge.sh` repite WordPress/backup/restore con Nginx real y Certbot temporal. Comprueba proxy DNS, challenge compartido, estado Docker, mounts, exit codes y rollback de validación/recarga sin cambiar default.conf. Certbot usa `--version` y ACME inalcanzable; no se afirma emisión pública probada. Pruebas unitarias adicionales cubren defaults nativos, aliases RO de systemd, lineages ajenas, certificados fuera de la raíz y cancelación de exec. La guía [Docker edge](docker-edge.md) describe la validación manual restante en Ubuntu.
