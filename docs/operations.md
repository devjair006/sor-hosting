# Operación y recuperación

## Sesiones y auditoría

No hay registro público. `create-admin` crea una cuenta con contraseña propia; Argon2id y cookies HttpOnly/Secure protegen el acceso. Logout y cambio de contraseña invalidan sesiones en servidor. Las escrituras exigen Origin y CSRF. La auditoría registra actor, recurso, acción y resultado sin contraseñas ni tokens.

## Trabajos

Una mutación devuelve un ID durable. La UI consulta su estado y la navegación no lo cancela. Reintentar una petición que perdió la respuesta conserva su Idempotency-Key. El agente conserva su journal; un trabajo incierto tras caída se marca `interrupted` y no se repite automáticamente. El agente registra el inventario antes de crear recursos y, tras un reinicio, observa IDs exactos por las etiquetas del proyecto; no vuelve a instalar por deducción. Primero inspecciona sus recursos y los registros del servicio; el nuevo intento debe ser explícito.

El worker del agente serializa operaciones y recupera mantenimiento/proxy antes de continuar. Si no logra recuperar, las nuevas mutaciones permanecen bloqueadas. No borres a mano journals, archivos `.rollback` ni filas de recovery sin revisar el estado real.

## WordPress y dominios

Cada sitio tiene contenedores, volúmenes y red independientes. El usuario inicial WordPress es `sor_admin`; también puede acceder mediante su correo. La base de datos no publica puerto al host; el web solo publica loopback para Nginx. Los archivos secretos no se devuelven por API.

Estados de instalación: `provisioning`, `pending_dns`, `pending_ssl`, `active`, `failed` y `restoring`. Un sitio instalado con DNS o SSL pendientes conserva sus recursos y permite comprobar el dominio después. La UI distingue configuración del sitio de su disponibilidad HTTPS observada. Si la instalación falla, se conservan inventario y datos para diagnóstico; no se borran volúmenes automáticamente.

El dominio del panel está protegido en la API. El agente también rechaza un hostname que ya pertenece a un vhost ajeno a su directorio gestionado. Para importar un sitio, el operador debe migrar primero su configuración de proxy de forma explícita.

Agregar un dominio no crea registros DNS. Comprueba A y AAAA: todos deben corresponder a IP públicas configuradas. El agente no intenta renovar otros certificados ni fuerza reemisiones fuera de elegibilidad.

## Backups

Backups manuales locales con manifest v1, archivos WordPress y dump MariaDB; checksum SHA-256 y validación previa a restaurar. Se detienen únicamente los escritores activos declarados; se recupera su estado anterior tanto en error como tras reiniciar el agente. El directorio está fuera del webroot. Descargas solo mediante sesión administrativa.

Antes de restaurar se comprueban checksum, rutas, symlinks, tamaño, sitio, dominio, versión y espacio. Se crea un snapshot obligatorio, se restaura en recursos nuevos y se cambia Nginx después de validar WordPress. Los recursos anteriores se conservan; no hay limpieza automática destructiva. Si falla el cambio, se recupera el destino anterior. Los escritores adicionales de sitios adoptados requieren migración al formato soportado para restauración.

Conserva copias externas de los backups y de la configuración/secretos operativos del agente. El formato inicial restaura el mismo sitio y dominio en esta instalación; no es una herramienta de migración a otra máquina o dominio. La retención y transferencia fuera del VPS son responsabilidad del operador en esta versión.

## Despliegue y rollback

`deploy/release.sh` recibe `SOR_IMAGE=ghcr.io/...@sha256:...`. Crea un snapshot SQLite coherente con `VACUUM INTO`, descarga la imagen, recrea el panel y comprueba readiness. No copia un archivo SQLite abierto. Conserva la imagen anterior y solo hace rollback automático si el esquema sigue siendo compatible; una migración irreversible requiere intervención y recuperación del snapshot.

El snapshot del panel se guarda en el volumen de datos, con nombre `predeploy-<UTC>.db`, y contiene información sensible: guarda permisos y retención adecuados. Readiness comprueba API/SQLite; las integraciones tienen sus propios estados visibles. Un panel ready no certifica por sí solo que WordPress y Certbot funcionan.

## Datos persistentes

- Panel: `SOR_DATA_DIR/panel.db`, WAL y snapshots predeploy.
- Agente: `/var/lib/sor-hosting/agent/journal.db` y clave externa en `/etc/sor-hosting/agent.key`.
- Plantillas/secretos por sitio: `/var/lib/sor-hosting/sites/<id>/`.
- Backups: `/var/lib/sor-hosting/backups/`.
- Vhosts gestionados: `/etc/nginx/sor-hosting.d/`.

La clave del panel cifra los payloads pendientes. Cambiarla sin completar o migrar esos trabajos impide recuperarlos. Antes de actualizar el agente, espera a que termine la cola y preserva un artefacto anterior compatible.
