# Instalar SOR Hosting en el VPS

Esta guía prepara una instalación nueva junto a Nginx, Certbot, Docker, node_exporter y Prometheus existentes. Los comandos de host se ejecutan por el operador en el VPS; no se han ejecutado automáticamente durante el desarrollo.

## 1. Preparar artefactos

Ejecuta `./tools/bootstrap.sh`, `make check`, `make build` en un equipo con Go y acceso a dependencias. Copia el repositorio y `bin/sor-agent` a un directorio de despliegue del VPS. El usuario/grupo numérico 10001 debe estar libre o reservado para este panel. El agente es un servicio con privilegios de root por sus tareas de Docker/Nginx/Certbot; no se expone por TCP.

Revisa `deploy/install-agent.sh --dry-run`. En el VPS, como root, ejecuta `./deploy/install-agent.sh`. No arranca el agente ni modifica nginx.conf. Conserva claves y configuración existentes al repetirse.

## 2. Configurar el agente

Edita `/etc/sor-hosting/agent.json`, creado desde `deploy/agent.example.json`. Define IP públicas reales, correo de Certbot, Docker data-root si difiere, y los IDs completos de contenedores que deban quedar protegidos. Nginx/Prometheus/node-exporter/panel tienen protecciones adicionales, pero registra también los IDs de infraestructura propia.

Configura `wordpress_image`, `mariadb_image` y `cli_image` con imágenes oficiales y **digests** compatibles de WordPress, MariaDB y WordPress CLI. No hay tags flotantes predeterminados. Puedes resolver el digest después de elegir y probar una versión:

```bash
docker pull wordpress:VERSION_ELEGIDA
docker image inspect --format '{{index .RepoDigests 0}}' wordpress:VERSION_ELEGIDA
```

Repite para MariaDB y WordPress CLI. La instalación usa permisos explícitos de secretos después de crearlos, para que el UMask restrictivo del agente no impida su lectura dentro de los contenedores.

La imagen MariaDB debe contener `healthcheck.sh`, `mariadb` y `mariadb-dump`. El entrypoint de WordPress debe admitir `WORDPRESS_DB_PASSWORD_FILE`; WordPress CLI debe admitir `wp core install --prompt=admin_password`. Prueba la combinación en un sitio desechable antes de usarla en producción. Fija la misma imagen MariaDB al restaurar; una migración de motor se hace por separado.

La clave `/etc/sor-hosting/agent.key` se genera localmente y no es la del panel. Conserva una copia protegida junto al journal y los secretos; sin ella no se recuperan operaciones pendientes cifradas. `peer_uid` y `peer_gid` deben coincidir con `10001:10001` de Compose. Docker rootless/userns-remap requieren ajustar el mapeo real de credenciales antes de esta instalación.

## 3. Integrar Nginx y Certbot

Haz copia de nginx.conf. Dentro de su bloque `http`, añade una vez el include de `deploy/nginx/sor-sites-include.conf`, conserva los demás vhosts y ejecuta `nginx -t`. Revisa y recarga Nginx. El agente solo escribe en `/etc/nginx/sor-hosting.d`.

El webroot `/var/lib/sor-hosting/acme` y sus padres deben ser transitables por Nginx. La instalación usa padre `0711`, webroot `0755` y directorios de secretos/backups `0700`.

Instala un vhost para el dominio del panel a partir de `deploy/nginx/panel.conf`, cambiando `server_name` y el puerto si procede. Apunta DNS y usa tu Certbot existente para habilitar HTTPS en ese vhost. La cookie de producción requiere HTTPS. Los certificados de sitios creados por el agente usan nombres `sor-<dominio>`; el certificado del panel se administra inicialmente desde el host.

Conserva tu timer/cron de renovación. Si necesitas un deploy hook, revisa `deploy/nginx/certbot-deploy-hook.sh` y añádelo al mecanismo de renovación existente; valida Nginx antes de recargar. No dupliques timers.

Arranca el agente: `systemctl enable --now sor-hosting-agent`. Comprueba el journal con `journalctl -u sor-hosting-agent`. Nunca publiques su socket ni configures un puerto TCP Docker abierto.

## 4. Configurar y arrancar el panel

Copia `deploy/.env.example` a `deploy/.env`, permisos `0600`. Genera una clave distinta con `openssl rand -base64 32` y rellena `SOR_SECRET_KEY`; fija `SOR_ORIGIN` al origen HTTPS exacto sin path final. Configura URL e instancia de Prometheus accesibles desde el contenedor: `localhost` dentro del contenedor no representa el VPS. Si Prometheus está en Docker, usa una red compartida explícita o su dirección interna autorizada, conservando sus restricciones de acceso.

Desde la raíz:

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml build panel
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d panel
docker compose --env-file deploy/.env -f deploy/compose.yaml exec panel /sor-server create-admin --email admin@tu-dominio.com
```

La última instrucción usa TTY para ocultar la contraseña. Para automatizar, usa `--password-file` con un secreto montado temporalmente y elimínalo al terminar. Nunca pases contraseñas por argv ni crees archivos con secretos en el repositorio.

Abre el dominio HTTPS del panel. Verifica sesión, agente conectado, las métricas de la instancia correcta y un sitio desechable completo antes de importar sitios con datos.

## 5. Adoptar sitios existentes

La adopción está limitada a WordPress/MariaDB compatibles con la plantilla operativa. Declara una entrada en `adoptions`, indexada por el ID Docker completo del web. Usa los campos de `SiteResources` del contrato compartido: dominio, web_id, db_id, writers, port, secret_ref, red/volúmenes, `resources: {memory_mb: 512, cpu_limit: 1}` (o su presupuesto real) y los datos declarados necesarios. La referencia secreta debe identificar archivos preparados por el operador bajo el directorio privado del agente; el contenedor DB debe montar el fichero cliente requerido en `/run/secrets/db_client`.

No se aceptan nombres como autorización ni rutas secretas enviadas por el navegador. Configura y revisa el vhost gestionado por SOR en `/etc/nginx/sor-hosting.d/<hostname>.conf` antes de habilitar restauración (conserva copia del vhost original fuera del include): no puede coexistir otro vhost que sirva el mismo dominio. La adopción por sí sola no modifica contenedores ni Nginx. Los sitios con escritores adicionales se pueden respaldar al declararlos, pero requieren migración a la plantilla soportada antes de restaurar mediante staging.

## 6. CI/CD

El workflow prueba, construye y publica GHCR desde `main`. El despliegue es manual mediante `workflow_dispatch` con `deploy=true`, en el environment protegido `production`. Configura los secrets `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS` y `DEPLOY_PATH`; el host debe poder descargar su imagen privada de GHCR. Elige permisos de publicación mínimos.

El flujo actual actualiza el contenedor del panel. El agente nativo se compila como artefacto del build y se actualiza por el operador durante una ventana sin trabajos activos, manteniendo binario anterior, configuración y journal para recuperación. No se reemplaza un agente en mitad de una restauración.

## Nginx y Certbot en Docker

Para contenedores existentes, sigue [la guía de modo Docker](docker-edge.md). Incluye JSON, montajes explícitos de systemd manteniendo ProtectHome, includes separados, red de proxy y rollback manual. El instalador nativo no migra servicios existentes ni aplica ese override.
