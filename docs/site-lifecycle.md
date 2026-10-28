# Sitios externos y eliminación segura

Esta versión no requiere adoptar los tres WordPress existentes ni modificar MariaDB, Nginx, Prometheus o Grafana. Ningún paso se ejecuta automáticamente en producción.

## Registrar un WordPress existente

En **WordPress → Registrar existente**, selecciona **Externo · solo lectura**. Introduce nombre y dominio; opcionalmente, los IDs completos de los contenedores WordPress y MariaDB. Puedes obtenerlos con `docker inspect --format '{{.Id}}' NOMBRE` en una sesión autorizada del VPS.

Varios sitios externos pueden referenciar la misma MariaDB. El agente solo inspecciona esas referencias y guarda metadatos: no ejecuta comandos dentro de los contenedores, no solicita contraseñas, no añade etiquetas y no cambia redes, volúmenes, certificados ni vhosts. El dashboard muestra disponibilidad HTTP(S), dominio, tipo y acceso a `/wp-admin`.

La API y el agente bloquean acciones Docker, dominios/SSL, eliminación, backups y restauraciones para externos. Los contenedores compartidos también quedan protegidos cuando otro registro administrado los referencia. La opción separada **Adopción completa** conserva el flujo anterior y exige una instalación compatible con credenciales y recursos exclusivos. Registrar como externo no convierte ni migra una instalación.

## Eliminar un sitio administrado

En el detalle del sitio, **Eliminar sitio** exige escribir el dominio completo, sin normalizar la confirmación. **Conservar backups** está activado por defecto. La operación necesita sesión, CSRF e `Idempotency-Key`; repetir la misma solicitud devuelve el mismo trabajo.

Solo se borran recursos que el agente puede demostrar que pertenecen exclusivamente al sitio instalado por SOR:

- Contenedores con IDs exactos, etiquetas SOR/Compose coincidentes y roles WordPress/MariaDB conocidos.
- Volúmenes locales propios, identificados por nombre, etiquetas y fecha de creación; no se usa borrado forzado. Se comprueban referencias incluso desde contenedores detenidos y otros sitios.
- Redes privadas de esos proyectos, sin referencias ajenas; nunca la red compartida del proxy.
- Archivos capturados del directorio Nginx administrado, con cabecera SOR y contenido sin cambios concurrentes.
- Certificados registrados por SOR con nombre y huella coincidentes y un único dominio. Certificados compartidos, protegidos o reemplazados se rechazan.

Los sitios adoptados, con escritores adicionales, protegidos o con propiedad incompleta no permiten eliminación automática. Si faltan recursos o no se puede obtener un backup consistente, primero hay que reparar el sitio. No existe una opción para forzar el borrado. No se emplean `docker prune`, limpieza global, borrado de bases compartidas ni modificaciones a `default.conf`.

### Fallos y recuperación

1. Se verifica el inventario, se guarda el plan durable y se crea un backup de seguridad. Se vuelve a verificar antes de suspender el sitio.
2. Se retiran sus includes Nginx y se detienen sus contenedores. Si falla esta preparación, el agente restaura los archivos capturados y el estado de ejecución anterior. Si el rollback falla, conserva el plan y bloquea mutaciones hasta poder reintentarlo.
3. Antes del primer borrado, se persiste la decisión de continuar. Desde ese punto los volúmenes eliminados no son reversibles: la recuperación reanuda los pasos pendientes, con comprobaciones de identidad, sin recrear datos vacíos. Se necesita el backup para restaurar contenido.
4. El estado final del sitio, trabajo y auditoría del agente se confirma en una transacción, junto con la eliminación del plan de recuperación. La API reconcilia ese mismo trabajo; un fallo al actualizar SQLite mantiene el trabajo pendiente para reintentar.

El registro del sitio queda como tombstone, oculto del inventario normal; su dominio puede registrarse de nuevo. El historial y auditoría se conservan. Si se conservan backups, siguen disponibles para descarga, incluido el de seguridad. Restaurar directamente al sitio eliminado está bloqueado; descarga el archivo y sigue la recuperación manual de [operaciones](operations.md) hacia una instalación nueva. Si no se conservan, se eliminan únicamente los archivos de backup registrados para ese sitio, al final del proceso. El agente retiene su estado privado de Compose y secretos para recuperación/inspección; no ejecuta una limpieza recursiva de directorios.

Un conflicto de propiedad durante la recuperación queda pendiente para revisión del operador. No borres las filas de recuperación ni uses limpieza global para desbloquearlo. Repara el montaje, configuración o acceso implicado y reinicia el agente; consulta el trabajo y la auditoría. El agente serializa estas operaciones; no hagas cambios manuales concurrentes a los recursos del sitio.

## API

`POST /api/v1/sites/external` recibe `name`, `domain` y opcionalmente `web_container_id` / `database_container_id` (64 caracteres hexadecimales). `POST /api/v1/sites/{id}/delete` recibe `confirm_domain` y `keep_backups`. Ambos devuelven `202` con un trabajo consultable. Contrato completo: [OpenAPI](api/openapi.yaml).

El inventario expone `management: managed|external`, `lifecycle: present|deleting|deleted`, `can_delete` y `can_backup`. Las capacidades visuales no sustituyen las comprobaciones del servidor y del agente.

## Actualización manual

1. Antes de actualizar, comprueba que no haya trabajos/recuperaciones pendientes. Guarda copias consistentes de ambas bases SQLite y del estado privado del agente, con sus permisos y clave de cifrado, además de backups de sitios fuera del VPS.
2. Descarga el artifact `sor-agent-linux-amd64` de una ejecución aprobada de GitHub Actions y verifica `SHA256SUMS` y `BUILD.txt`. Instala primero el agente y luego la API usando el procedimiento de [Docker edge](docker-edge.md), respetando los montajes explícitos y `ProtectHome`. No cambies los servicios WordPress existentes.
3. Ambas bases migran al esquema 2 al arrancar. El agente 0.3 anuncia `site_lifecycle`; una API nueva rechaza estas dos operaciones si el agente antiguo no lo anuncia. Verifica el estado antes de registrar sitios externos.
4. La migración de SQLite no tiene downgrade automático. Para revertir binarios a una versión de esquema 1, detén API/agente y restaura las copias compatibles de ambas bases/estado. No restaures una instantánea anterior a una eliminación confirmada sin reconciliar los recursos reales; no recuperará volúmenes borrados.

No hay despliegue a producción desde esta rama. Publicar un artifact no instala el agente en el VPS.

## Pruebas reproducibles

`make check build` y `python3 tools/check-schema.py` cubren Go, SQL, tipos y build React. `npm run test:e2e --prefix frontend` verifica registro externo, permisos visuales, confirmación exacta y retención por defecto; `npm run test:live --prefix frontend` usa autenticación Go y SQLite reales.

En una máquina de pruebas con Docker, ejecutar por separado:

```bash
SOR_LIFECYCLE_INTEGRATION=1 SOR_LIFECYCLE_KEEP_BACKUPS=0 bash tools/test-docker.sh
SOR_LIFECYCLE_INTEGRATION=1 SOR_LIFECYCLE_KEEP_BACKUPS=1 bash tools/test-docker-edge.sh
```

Estas pruebas crean proyectos aislados con IDs aleatorios. Verifican rechazo de recursos compartidos/protegidos, rollback de Nginx, recuperación tras un error de Docker posterior a borrados parciales, idempotencia, auditoría y ambas políticas de backups. El modo edge usa un certificado privado de prueba y Certbot real sin solicitudes a una CA pública. La limpieza se limita a los recursos de esas fixtures.
