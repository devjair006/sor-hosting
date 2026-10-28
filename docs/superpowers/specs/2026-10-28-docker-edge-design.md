# Nginx y Certbot en Docker

Solicitud ejecutable del usuario: añadir modo Docker manteniendo el nativo; entregar código, pruebas, artifact Linux amd64 y despliegue manual. No acceder al VPS ni modificar producción.

El modo nativo actual seguirá siendo el predeterminado. Se añaden opciones `nginx` y `certbot` al JSON del agente, con rutas distintas para el host del daemon, el namespace systemd del agente y el interior de Nginx/Certbot. No se ejecutan comandos arbitrarios ni se escribe default.conf.

## Enfoque

Se considera Docker CLI como alternativa, pero se usa el SDK Moby existente para exec/inspect y contenedores temporales: conserva identidad exacta, códigos de salida, límites y cancelación. Migrar Nginx al host o cambiar sus vhosts queda fuera del alcance pedido.

Nginx usa exec `nginx -T`, `-t` y `-s reload` sobre el ID inspeccionado del contenedor configurado. Su estado procede de Docker Inspect, no del PID del host. Se validan los mounts compartidos antes de escribir y se conserva el journal de rollback. El directorio de includes tiene una ruta del host y otra dentro del contenedor; se rechazan vhosts no administrados y dominios protegidos. Los sitios nuevos conectan solo su web a una red externa de proxy explícita; MariaDB permanece aislada. El upstream se obtiene del endpoint de esa red compartida y se actualiza al restaurar.

Certbot usa imagen por digest, mounts explícitos de la configuración y webroot existentes, sin puertos ni docker.sock; work/logs temporales. Lee certificados desde una ruta del agente que puede ser un bind mount de systemd. Solo opera sobre nombres `sor-<dominio>`; protege los dominios configurados y rechaza lineages existentes sin registro de propiedad en el diario SOR. No ejecuta directory hooks ajenos. Nunca renueva todas las lineages.

`ProtectHome=true` se conserva. El ejemplo expone únicamente certificados y webroot por BindReadOnlyPaths/BindPaths fuera de /home. El daemon recibe los sources de host explícitos; el agente nunca necesita abrir esos sources bajo /home. La habilitación de mounts/includes/red y reinicios son pasos manuales y reversibles.

## Pruebas y entrega

Pruebas nativas existentes, validación de configuración/rutas, protección de nombres/certificados, exec con salida/error/cancelación y rollback con Nginx Docker real en un entorno local desechable. Certbot real en contenedor para versión/errores/mounts y fixture de certificado para lectura; emisión pública ACME requiere validación manual de staging. WordPress/backup/restore se vuelven a probar. CI publica `sor-agent-linux-amd64` con SHA256 por upload-artifact; ninguna ejecución nueva despliega en Azure.
