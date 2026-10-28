# Nginx y Certbot en Docker: despliegue manual

Esta guía adapta el agente systemd a `wordpress-server-nginx-1` en Ubuntu x86_64. **No se ha ejecutado sobre el VPS.** La API conserva su socket local y su configuración; el origen del panel sigue siendo `https://sorpanel.projectkarli.dev`.

## Qué cambia

`nginx.mode` y `certbot.mode` aceptan `native` (predeterminado) o `docker`. Los JSON antiguos continúan usando los ejecutables y PID de Nginx del host. En Docker, el agente inspecciona el contenedor configurado y ejecuta únicamente `nginx -T`, `nginx -t` y `nginx -s reload` mediante Moby Exec API, comprobando el exit code. No usa el PID del host ni un shell.

Las operaciones genéricas de Docker deniegan Nginx por su nombre configurado, incluso si se recrea con otro ID. Infraestructura no registrada no se vuelve administrable por activar este modo. Los sitios existentes siguen requiriendo aprobación explícita para adopción; no se reinician ni migran automáticamente.

Los certificados usan nombres `sor-<dominio>` y no pueden sobrescribir una lineage ya existente sin asociación al certificado registrado en el diario SOR. `protected_domains` bloquea emisión, renovación y vhosts de los dominios declarados. También se detectan conflictos con los vhosts no administrados del `nginx -T` real. Una emisión completada pero sin registro en el diario por una interrupción requiere revisión manual; no se reclama automáticamente su lineage.

## Rutas: tres namespaces

| Recurso | Host del daemon Docker | Agente systemd | Contenedor |
|---|---|---|---|
| Includes SOR nuevos | `/etc/nginx/sor-hosting.d` | misma ruta | `/etc/nginx/sor-hosting.d` en Nginx, RO |
| Certificados existentes | `/home/azureuser/wordpress-server/certbot/conf` | `/var/lib/sor-hosting/certificates`, bind RO | `/etc/letsencrypt` |
| ACME webroot existente | `/home/azureuser/wordpress-server/certbot/www` | `/var/lib/sor-hosting/acme`, bind explícito | `/var/www/certbot` |
| Vhosts existentes | `/home/azureuser/wordpress-server/nginx/default.conf` | no se abre | mount actual de Nginx, intacto |

`certbot.directory` es la ruta **legible por el agente**; `config_source` y `webroot_source` son los sources **del host del daemon**, pasados explícitamente a Docker. No apuntes el source de Docker a un alias que solo existe dentro del namespace systemd. Los symlinks de `live` a `archive` se admiten únicamente dentro de la raíz configurada. Cuando el alias y source difieren, el agente compara `/proc/self/mountinfo` con `/proc/1/mountinfo`: exige el bind RO del source exacto antes de permitir operaciones Certbot. Una carpeta vacía sin montar no sirve como alias.

`ProtectHome=true` permanece activo. El agente rechaza rutas de lectura/escritura bajo `/home`, `/root` y `/run/user`; expón únicamente las subcarpetas necesarias mediante el drop-in proporcionado. No desactives ProtectHome ni montes toda la carpeta de azureuser. Esto usa los [bind mounts de systemd](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#BindPaths=).

## Preparación y revisión sin modificar servicios

1. Descarga el artifact `sor-agent-linux-amd64` de la ejecución de GitHub Actions de esta rama y comprueba `sha256sum -c SHA256SUMS`. El ZIP contiene el binario, checksum y commit en `BUILD.txt`. Guarda una copia del binario anterior. El artifact no se instala ni se despliega por sí solo.
2. En el VPS, inspecciona `docker inspect wordpress-server-nginx-1` y `docker exec wordpress-server-nginx-1 nginx -T`. Comprueba rutas de mounts, todas sus redes actuales, contexto `http` de `conf.d/*.conf`, nombres de servidor y rutas TLS. Conserva un backup de los archivos de Compose y Nginx antes de preparar un override. No edites `default.conf`.
3. Compara `deploy/agent.docker.example.json` con esas rutas. Configura IP pública real, email, imágenes WordPress/MariaDB/CLI y Certbot **por digest**. El campo Certbot image queda vacío a propósito. Descarga manualmente la imagen aprobada por digest; el agente no hace pull implícito. Revisa la lista de dominios protegidos contra el `nginx -T`: `pagina1`/`pagina2` pueden tener otros FQDN; los vhosts existentes se protegen también por detección real.
4. Verifica que el origen HTTPS del panel ya configurado en la API sea `https://sorpanel.projectkarli.dev`. Conserva su vhost actual; este cambio no requiere reemplazarlo.

## Habilitación manual en ventana de mantenimiento

Los comandos siguientes son pasos para el operador; no los ejecutó la implementación.

```bash
# Solo directorios nuevos fuera de /home; no cambia propietarios de las carpetas existentes.
sudo install -d -m 0755 /etc/nginx/sor-hosting.d
sudo install -d -m 0700 /etc/sor-hosting /var/lib/sor-hosting/certificates
sudo install -d -m 0755 /var/lib/sor-hosting/acme
sudo install -m 0644 deploy/nginx/docker-managed-include.conf /etc/sor-hosting/nginx-managed-include.conf
sudo docker network create sor-proxy
```

Si la red ya existe, inspecciónala y verifica que sea una red bridge dedicada; no la reemplaces. Nginx necesita esa red además de **todas** sus redes anteriores. Cada web nueva se conecta a ella con un alias único; MariaDB conserva exclusivamente la red del sitio. El proxy usa el resolver Docker `127.0.0.11`, evitando IPs de contenedor obsoletas después de reiniciar o restaurar.

Prepara una copia local del override de `deploy/nginx/docker-compose.override.example.yaml`. Su clave de servicio `nginx` debe coincidir con el Compose original. El ejemplo conserva la red `default`; si hay redes custom, conserva cada una explícitamente. Revisa `docker compose -f <compose-original> -f <override> config`: **no deben desaparecer volúmenes, puertos ni redes actuales**. El override solo añade dos mounts y la red del proxy; no contiene ninguna definición para WordPress, MariaDB, Prometheus ni Grafana.

Solo el operador puede aplicar después ese override con `docker compose ... up -d --no-deps nginx`. Esto recrea Nginx y puede interrumpir tráfico brevemente; no es una acción del agente. El include se monta como **un archivo nuevo** en `/etc/nginx/conf.d/zz-sor-hosting.conf`, nunca sobre `default.conf`. Si la configuración global no incluye `conf.d/*.conf` dentro de `http`, detente y habilita un include separado en la configuración global revisada; no cambies los vhosts.

Comprueba `docker exec wordpress-server-nginx-1 nginx -T` y `nginx -t`. Si Nginx pide aumentar `server_names_hash_bucket_size` para dominios largos, revísalo en su configuración global `http` (128 funcionó en el fixture), evitando duplicar una directiva ya definida. El agente rechaza el cambio y restaura el archivo anterior; no ajusta valores globales automáticamente.

```bash
# Montajes EXPLÍCITOS para el agente, sin desproteger /home.
sudo install -d -m 0755 /etc/systemd/system/sor-hosting-agent.service.d
sudo install -m 0644 deploy/systemd/sor-hosting-agent.service.d/docker-edge.conf.example \
  /etc/systemd/system/sor-hosting-agent.service.d/docker-edge.conf
# Copia agent.docker.example.json a un archivo de preparación, configura valores reales,
# revisa el diff con agent.json y conserva el original antes de sustituirlo.
```

Comprueba los sources reales antes de aplicar el drop-in. El webroot debe seguir siendo legible por los workers de Nginx; no cambies permisos recursivamente ni copies certificados a otra raíz. El montaje RO de certificados del agente mantiene juntos `live` y `archive`.

Cuando no haya trabajos activos, detén únicamente el agente, respalda su SQLite y WAL de forma coherente y guarda el JSON anterior. Instala el nuevo binario con `install -m 0755 sor-agent-linux-amd64 /usr/local/sbin/sor-hosting-agent`, coloca el JSON revisado en `/etc/sor-hosting/agent.json`, ejecuta `systemctl daemon-reload` e inicia únicamente `sor-hosting-agent`. Revisa `systemctl cat`, `systemctl status` y el journal antes de habilitar operaciones desde el panel. El usuario de la API y sus UID/GID permanecen 10001.

## Certbot temporal

Se ejecuta `certonly --webroot` o una renovación selectiva `renew --cert-name sor-...`. No se exponen puertos ni docker.sock; no se usa el plugin nginx ni se ejecutan directory hooks existentes. La configuración y webroot existentes se montan explícitamente RW; work/logs/tmp son temporales. La raíz del contenedor es RO, no es privileged y tiene límites de CPU/RAM/PIDs. Se elimina al terminar, fallar o cancelarse.

Solo se añade `DAC_OVERRIDE` tras eliminar las otras capabilities: permite escribir el challenge en un webroot que pertenece a azureuser sin cambiar sus permisos. Los permisos/propietarios del almacén Certbot deben ser compatibles con el Certbot root que ya usa la infraestructura. El contenedor ve exclusivamente los dos mounts declarados; el agente ve certificados por el alias RO. La [documentación de Certbot Docker](https://eff-certbot.readthedocs.io/en/stable/install.html#alternative-1-docker) describe la separación entre obtener certificados e instalarlos en el servidor web.

Tras una terminación forzada del agente, revisa los contenedores con label `sor.hosting/certbot=true` y sus IDs antes de reiniciar operaciones. No elimines otro Certbot ni uses `docker prune`. El bloqueo de Certbot evita escritores concurrentes sobre su configuración. Para validar emisión real usa primero un dominio de prueba no protegido y `certbot.server=https://acme-staging-v02.api.letsencrypt.org/directory`; un certificado staging no debe sustituir uno de producción. Retira el override de server antes de emitir producción sobre una lineage separada/revisada.

## Verificaciones y rollback

- Comprueba disponibilidad real del contenedor y acceso a las rutas existentes de projectkarli.dev, pagina1, pagina2 y al panel, antes y después.
- Instala un sitio nuevo de prueba; comprueba proxy, ACME HTTP-01, certificado y acceso wp-admin. No adoptes ni reinicies los tres sitios actuales para validar el agente.
- Prueba backup/restore de ese sitio desechable. La restauración cambia el alias al staging; si falla el cutover, el rollback restaura las rutas anteriores y conserva los recursos previos.
- Un `nginx -t` fallido restaura el archivo anterior sin recargar la configuración inválida. Una recarga fallida intenta volver al archivo y recarga anteriores. Si esa recuperación no es posible, conserva `.sor-*.rollback`; el agente bloqueará operaciones hasta recuperar. No borres esos markers a ciegas.
- Para retirar esta versión: drena trabajos, detén el agente, restaura binario y JSON anteriores y retira solo su drop-in nuevo. Recarga systemd e inicia el agente en modo nativo únicamente si Nginx/Certbot nativos existen realmente. Si la infraestructura sigue en Docker, conserva detenidas las operaciones incompatibles del agente antiguo. No elimines los includes o la red que usen sitios nuevos activos; desmontarlos requiere primero su migración manual. El rollback del binario no revierte bases de datos WordPress ni borra certificados.

## Pruebas locales y CI

```bash
make check build
bash tools/test-docker.sh       # regresión nativa con proxy controlado
bash tools/test-docker-edge.sh  # Nginx y Certbot reales, proyectos locales desechables
```

La integración Docker comprueba instalación/restauración WordPress, aislamiento de DB, proxy por DNS Docker, estado de Nginx parado/arrancado, webroot compartido, protección de vhosts, mounts erróneos sin escritura, exit codes de exec, errores de validación/recarga con rollback y limpieza de Certbot. Certbot real se prueba con `--version` y un endpoint ACME local inalcanzable; no se solicita un certificado público. La lectura de certificados y confinamiento de symlinks tiene pruebas con certificados generados localmente. La emisión ACME pública y el montaje systemd en Ubuntu se validan manualmente en staging; estas pruebas locales no se presentan como validación del VPS.
