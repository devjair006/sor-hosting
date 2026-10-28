# SOR Hosting: diseño técnico y de producto

Fecha: 2026-10-27. Estado: aprobado por el usuario («si me encanta»); listo para planificar la implementación.

## Intención y acuerdos

Crear un panel privado, en español, para administrar el VPS Linux del usuario en Azure. El servidor ya tiene Nginx, Certbot, Docker Engine, node_exporter y Prometheus. SOR Hosting reúne operaciones de WordPress, contenedores, dominios, certificados, métricas y backups en una interfaz propia. No utiliza la marca ni el software comercial cPanel.

El usuario definió el stack y la estructura inicial, eligió el nombre SOR Hosting y confirmó API en Docker más agente Go nativo en el VPS. Confirmó también una identidad de grafito, marfil y esmeralda, sin degradados. La carpeta es `sor-hosting/`.

## Alcance y límites

Primera versión: un VPS y acceso de administradores, con login, dashboard, sitios WordPress, Docker Manager, dominios y SSL, instalador WordPress y backups con restauración. Incluye auditoría, trabajos asíncronos, documentación de instalación y CI/CD.

La tabla de servidores prepara la asociación de recursos, pero no implica administración remota de varios hosts en esta versión. Se podrá importar explícitamente un WordPress existente si usa contenedores Docker y una base de datos compatible con la plantilla soportada.

Quedan fuera de esta versión: facturación, correo, FTP, terminal web, ejecución libre de comandos, panel de clientes, registro público, gestión de Azure, alojamiento no basado en Docker y administración de zonas DNS mediante API. Agregar un dominio al panel configura el alojamiento; sus registros DNS se crean en el proveedor del usuario.

Es un fallo de producto afectar sitios o infraestructura ajenos al panel, mostrar datos simulados como reales o declarar exitoso un trabajo que no terminó.

## Enfoques considerados

| Enfoque | Ventaja | Coste | Decisión |
|---|---|---|---|
| API en Docker y agente Go nativo | Separa el panel público de las operaciones locales de Docker, Nginx y Certbot; encaja con Compose | Requiere instalar y mantener un servicio systemd y un socket local | Elegido por el usuario |
| API y agente nativos | Comunicación local sencilla | Despliegue de la API más acoplado al host | No elegido |

La selección evita montar el socket Docker o las carpetas de Nginx y Certbot en el contenedor de la API. El agente sigue siendo un servicio de confianza con privilegios de host; la separación reduce la superficie expuesta, pero no elimina ese privilegio.

## Arquitectura

Flujo: navegador → Nginx existente con HTTPS → API Go/Chi y frontend compilado → SQLite, Prometheus y agente local → Docker Engine/Nginx/Certbot.

- Un contenedor sirve la API `/api/v1` y los archivos Vite. Nginx publica el panel bajo un dominio configurado. La API escucha en un puerto publicado solo en `127.0.0.1`.
- SQLite vive en un volumen persistente. Una instancia de API, WAL, claves foráneas, `busy_timeout` y escritura coordinada. Las migraciones se aplican antes de aceptar tráfico; un fallo de migración impide arrancar.
- El agente es un binario Go bajo systemd. Escucha solo en `/run/sor-hosting/agent.sock`, con permisos de grupo para el UID/GID de la API. Compose monta únicamente el directorio de ese socket. No se publica una API del agente por TCP.
- El agente verifica el UID/GID del cliente mediante credenciales del socket Unix. No ofrece shell, comandos arbitrarios ni proxy genérico del Docker API. Implementa operaciones tipadas y valida sus argumentos.
- El agente usa Docker Engine Go SDK; la API conserva autorización de usuarios, sesiones, inventario, auditoría y estado de los trabajos.
- Prometheus se consulta desde el backend por una URL de configuración, con destino y selector de instancia definidos por el operador. No admite URLs arbitrarias enviadas desde el navegador.

### Estructura

Se conserva la estructura solicitada y se amplía por responsabilidad:

```text
sor-hosting/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── auth/ sites/ containers/ domains/ ssl/ monitoring/ database/
│   │   ├── agentclient/ backups/ jobs/ audit/ config/ httpapi/
│   ├── migrations/
│   ├── queries/
│   ├── sqlc.yaml
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/ui/
│   │   ├── components/layout/
│   │   ├── pages/ hooks/ services/ lib/
│   ├── package.json
│   └── vite.config.ts
├── agent/
│   ├── cmd/agent/main.go
│   ├── internal/docker/ nginx/ certbot/ wordpress/ backups/ rpc/
│   └── go.mod
├── deploy/
│   ├── Dockerfile
│   ├── compose.yaml
│   ├── nginx/ systemd/
│   └── .env.example
├── design-system/sor-hosting/MASTER.md
├── docs/superpowers/specs/
└── .github/workflows/deploy.yml
```

Los nombres agrupados con `/` representan directorios separados. Los detalles de archivos y firmas corresponden al plan de implementación.

## Stack

- Backend y agente: Go, Chi v5, `net/http`, `modernc.org/sqlite` sin CGO y consultas generadas por sqlc para `database/sql`.
- Frontend: React, TypeScript estricto, Vite, Tailwind CSS y componentes reales de shadcn/ui adaptados a SOR. React Router para enlaces directos y navegación; TanStack Query para solicitudes, estados y refresco. Lucide como única familia de iconos. Recharts para series temporales.
- Sesiones persistidas en SQLite, cookies seguras y Argon2id. Docker Engine Go SDK con negociación de versión del API.
- Despliegue: Compose, imagen en GHCR y GitHub Actions. Las versiones exactas se fijan en los manifiestos y lockfiles tras comprobar compatibilidad al implementar.

## Identidad y sistema visual

Fuente de verdad: `design-system/sor-hosting/MASTER.md`. UI/UX Pro Max aportó recomendaciones verificadas de minimalismo funcional, tipografía para datos, tablas, diálogos accesibles y series temporales. Su generador también devolvió una composición de landing page ajena a este producto después de un reintento: esa composición no se adopta. La distribución del panel y la paleta acordada son decisiones específicas de SOR Hosting.

- Identidad: monograma geométrico SOR con dos trazos angulares que evocan racks y una ruta ascendente. Logotipo vectorial propio, legible a 24 px; nombre SOR Hosting visible en navegación y login.
- Tema inicial: barra lateral grafito, contenido marfil, tarjetas blancas y acentos esmeralda. Tokens preparados para tema oscuro sin prometer un selector adicional en esta primera versión.
- Nada de degradados: tampoco en gráficas, ilustraciones, estados de carga, botones o logotipo. Bordes finos y sombras mínimas.
- Fira Sans para interfaz y Fira Code para IP, puertos, logs y cifras técnicas. Fuentes servidas localmente. Datos tabulares alineados.
- Diseño prioritario de escritorio, adaptable a móvil: sidebar de 248 px; barra superior de 72 px; contenido con márgenes de 32 px; escala de espaciado de 4/8 px; radios de 8–12 px.
- Una acción primaria por pantalla. Estados identificados mediante texto e icono además del color. Foco visible, uso completo con teclado y `prefers-reduced-motion`.

## Navegación y pantallas

Navegación principal: Resumen, Sitios WordPress, Contenedores, Dominios y SSL, Backups. El botón «Crear sitio» abre el instalador. Al pie: Auditoría, Configuración y cuenta con cierre de sesión. Encabezado con breadcrumb, servidor actual, estado de conexión y última actualización.

| Pantalla | Información y comportamiento |
|---|---|
| Login | Logo y texto breve, correo, contraseña, mostrar/ocultar, envío con estado de carga, errores genéricos de acceso; permite pegar y usar gestores de contraseñas |
| Resumen | CPU, RAM, disco y tráfico; historial 1 h / 6 h / 24 h; estado de servicios y sitios; actividad reciente; avisos de certificados o backups |
| Sitios WordPress | Búsqueda, filtros y tabla de dominio, estado, contenedores, SSL y último backup; enlaces públicos y `/wp-admin` sin login automático |
| Detalle de sitio | Resumen, contenedores asociados, dominios, certificados, backups y actividad; acciones específicas al sitio |
| Contenedores | Nombre, imagen, estado, CPU/RAM disponibles; iniciar, detener y reiniciar según permiso; logs con límite de líneas y actualización pausada por defecto |
| Dominios y SSL | Dominio, destino, comprobación DNS, certificado y vencimiento; agregar dominio, comprobar y solicitar renovación |
| Crear sitio | Asistente de dominio → WordPress → recursos → revisión; validación por paso, volver sin perder datos y progreso de la operación |
| Backups | Sitio, fecha, tamaño, estado, creación manual, descarga autorizada y restauración; información explícita del periodo de mantenimiento |
| Auditoría | Actor, acción, recurso, fecha y resultado; filtros y paginación |
| Configuración | Nombre del servidor, estado de conexiones, parámetros operativos no secretos y cambio de contraseña; rutas y destinos sensibles se configuran en el despliegue |

Los listados incluyen carga, vacío con siguiente paso, error con reintento y desconexión. Una integración ausente muestra «Sin conexión» o «Sin datos», nunca métricas inventadas. La vista previa incluida es un mockup rotulado, con datos ilustrativos.

## Login y autorización

- Sin registro público. Alta inicial mediante comando de la API con entrada de contraseña interactiva o archivo de secreto; nunca contraseña en argumentos del proceso ni cuenta predeterminada.
- Administradores con igual alcance en la primera versión. Los endpoints verifican sesión y permisos; ocultar un botón no es autorización. Se reserva un campo de rol para evolución posterior.
- Argon2id: sal aleatoria por contraseña, formato PHC y parámetros iniciales de 64 MiB, 3 iteraciones y paralelismo 1, con límite de concurrencia de verificaciones y validación estricta al leer hashes. Contraseñas de 12–128 caracteres, permitiendo espacios, pegado y gestores.
- Sesión con token aleatorio de 32 bytes; solo su hash queda en SQLite. Rotación al entrar, invalidación al salir o cambiar contraseña, caducidad por inactividad de 30 minutos y absoluta de 12 horas.
- Cookie de producción `__Host-sor_session`, `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, sin `Domain`. Modo HTTP local solo mediante configuración explícita de desarrollo con nombre distinto.
- Protección CSRF en escrituras con token asociado a sesión y comprobación de `Origin`. Sin CORS permisivo. Límite de intentos por IP y cuenta, cuerpo limitado y mensajes genéricos.
- Se confía en cabeceras reenviadas exclusivamente desde Nginx configurado. Logs, auditoría y respuestas nunca incluyen contraseñas, cookies, tokens ni claves privadas.

## Modelo de datos

IDs opacos, fechas UTC y claves foráneas. Entidades: `users`, `sessions`, `servers`, `sites`, `site_containers`, `domains`, `certificates`, `backups`, `jobs` y `audit_events`.

- `sites` pertenece a un servidor y conserva slug único, estado, dominio principal y referencias operativas sin contraseñas en claro.
- `site_containers` vincula identificadores Docker exactos, sitio y función; los nombres visibles no otorgan permisos.
- Dominio normalizado y único por servidor. Un certificado tiene nombres asociados, emisor, fingerprint, vigencia y referencia de Certbot.
- `jobs` tiene tipo, recurso, actor, estado, fase, clave de idempotencia, fechas y resultado saneado. `audit_events` es append-only desde el API.
- Contraseñas de WordPress, bases de datos y material necesario para trabajos se almacenan cifrados mediante AEAD con clave externa a SQLite. Se eliminan de los trabajos al terminar; secretos operativos persistentes viven en archivos del agente `0600` referenciados por ID.

## API y trabajos

API versionada `/api/v1`; objetos JSON con errores estables `{error: {code, message, request_id}}`. Endpoints de lectura paginados; no se devuelve HTML de errores internos. Los esquemas de peticiones y respuestas se fijan en el plan.

Familias: `/auth/login`, `/auth/logout`, `/auth/me`, `/auth/csrf`, `/dashboard`, `/monitoring`, `/sites`, `/containers`, `/domains`, `/certificates`, `/backups`, `/jobs` y `/audit`. Salud de proceso y readiness se publican por separado sin detalles sensibles.

Las mutaciones de infraestructura devuelven `202` con un ID de trabajo; la UI consulta su estado. Estados: `queued`, `running`, `succeeded`, `failed`, `interrupted`. Los jobs persisten y sus transiciones incluyen auditoría. No hay ejecución en una goroutine sin registro durable.

Las operaciones llevan un ID idempotente desde la API hasta el agente. El agente conserva un journal local de ejecuciones por ID y el resultado saneado, incluso si la API pierde la respuesta. Al reiniciar se reconcilian trabajos con ese journal y con el estado observado; un paso incierto no se reejecuta automáticamente. La UI ofrece reintento explícito cuando resulta seguro. El journal queda fuera de SQLite del panel, bajo `/var/lib/sor-hosting/agent/`.

Un bloqueo por sitio evita restaurar, respaldar e instalar simultáneamente; cambios de configuración de Nginx y Certbot se serializan en el agente. Cancelar la navegación no cancela un trabajo iniciado. No se promete cancelación de operaciones de infraestructura en esta versión.

## Operaciones sobre el VPS

### Contenedores y sitios existentes

El inventario puede mostrar todos los contenedores en modo lectura. Escribir, leer logs o respaldar exige que el recurso esté creado por SOR o adoptado explícitamente con asociación persistida y autorización local del agente. Los recursos adoptados quedan en su inventario local por ID exacto; recibir solo un ID desde la API no basta. Nginx, Prometheus y el propio panel quedan protegidos de operaciones de parada desde esta UI.

La adopción registra contenedores, motor y base de datos, volúmenes y configuración declarada, con comprobación previa. No modifica ni reinicia recursos. El agente aplica timeouts, límites de salida y redacción de credenciales conocidas. Los logs siguen siendo datos sensibles visibles solo a administradores autorizados; no se promete detectar todos los secretos que una aplicación imprima.

### Instalador WordPress

Plantilla inicial: WordPress con MariaDB por sitio, imágenes oficiales fijadas al implementar, volúmenes nombrados, red dedicada y credenciales aleatorias. Las bases de datos de WordPress son independientes de SQLite del panel. El sitio necesita correo y contraseña de administrador propios; los secretos no reaparecen después de completar la instalación.

El agente reserva un puerto de loopback y lo verifica con Docker; no publica la base de datos al host. Arranca la base de datos, espera readiness, inicia WordPress y usa WP-CLI con argumentos tipados para completar la instalación. No concatena datos del usuario en un shell. Cada recurso lleva etiquetas de propiedad y el ID del job.

La revisión final muestra dominio, recursos y alcance antes de crear. Se comprueba DNS para el paso SSL; si DNS no apunta al host, el sitio conserva estado «Pendiente de DNS», con los recursos creados y un reintento de SSL separado. Un fallo de provisión conserva un diagnóstico y limpia solo recursos creados por ese job cuando sea seguro, sin borrar datos adoptados.

### Nginx, dominios y certificados

Rutas configurables al instalar: directorio exclusivo de configuración SOR incluido por el Nginx existente, webroot ACME y referencias de Certbot. Se valida que los directorios están permitidos y no atraviesan symlinks no autorizados. No se reescriben archivos de otros sitios.

Solo FQDN válidos normalizados a ASCII/IDNA, sin puertos, rutas, comodines ni fragmentos de configuración. Se comprueba resolución A/AAAA y se explica cualquier discrepancia; no se crean registros DNS automáticamente.

Escritura temporal, validación completa `nginx -t`, reemplazo controlado y reload; ante fallo, rollback al archivo previo y diagnóstico. El agente usa ejecutables y argumentos fijos. Obtención HTTP-01 con webroot y `certbot certonly`; renovación selectiva mediante el cert-name registrado. El temporizador de renovación existente se conserva y se documenta un deploy hook que valida y recarga Nginx. «Renovar» respeta la elegibilidad de Certbot y no fuerza reemisiones por defecto.

La UI diferencia DNS, configuración HTTP, certificado emitido y HTTPS verificado; un certificado existente no basta para declarar accesibilidad pública del sitio.

### Métricas

CPU, RAM, disco y tráfico se consultan a Prometheus con selector de instancia configurado. Series a resolución acotada, con intervalo 1 h / 6 h / 24 h y refresco de 30 segundos, pausa y hora del último dato. CPU y tráfico usan tasas sobre contadores; RAM usa memoria disponible; disco excluye montajes efímeros y permite elegir el filesystem principal.

`up` de Prometheus indica disponibilidad del target, no garantiza que WordPress sirva correctamente. La salud de sitios se comprueba mediante HTTP con timeout y destinos previamente registrados; se bloquean redirecciones a destinos internos o no permitidos. Las métricas por contenedor vienen del Docker SDK; node_exporter no se presenta como fuente de métricas individuales de contenedores. No se añade cAdvisor como dependencia implícita.

### Backups y restauración

Destino inicial: directorio local fuera del webroot, `/var/lib/sor-hosting/backups`, con permisos restringidos y ruta configurable solo al instalar. Backup manual por sitio; programación y almacenamiento en Azure Blob quedan fuera de la primera versión. Los backups locales no protegen ante pérdida del VPS; la UI y la guía indican esta limitación en el contexto de elegir dónde guardar el respaldo.

Formato: manifest versionado, dump lógico de la base de datos, archivos WordPress y metadatos necesarios, con checksum y estados verificables. Durante la captura se detiene temporalmente el contenedor web y los escritores controlados del sitio, se espera el cierre de conexiones y se realiza el dump y la copia de archivos; un `defer`/fase de recuperación arranca los servicios tanto en éxito como en fallo. Un sitio adoptado requiere declarar y detener sus escritores adicionales o se rechaza la captura consistente. La UI informa del mantenimiento antes de empezar.

Restauración solo de backups registrados, del mismo sitio y formato soportado. Antes de tocar el sitio se verifica checksum, manifest, rutas de archivo, symlinks, tamaños y espacio libre. No hay carga de archivos arbitrarios ni extracción en rutas elegidas por el navegador.

Confirmación escribiendo el dominio, comprobación previa y snapshot de seguridad obligatorio; si falla el snapshot no se continúa. La restauración usa contenedores y volúmenes de staging, valida WordPress, conserva un manifiesto operativo de rollback y solo después cambia el destino del proxy. La configuración y el destino previos se conservan hasta comprobar el nuevo estado. El sitio conserva su dominio y URL; esta versión no promete migración a dominios diferentes. La descarga pasa por la API con autorización y streaming; no hay enlaces públicos a archivos.

## Despliegue y operación

Dockerfile multi-stage: Vite produce estáticos; Go compila con `CGO_ENABLED=0`; imagen de runtime sin herramientas de compilación, usuario sin privilegios y límites de recursos. Volumen SQLite y montaje del socket del agente. Secretos fuera del repositorio y ejemplos sin valores reales.

El agente requiere servicio systemd y directorios locales provisionados por una guía o script idempotente revisable. Su journal y secretos persisten fuera del contenedor de API. El cambio inicial que incluye el directorio SOR en Nginx se hace durante instalación con copia de seguridad y validación.

GitHub Actions ejecuta generación de sqlc sin diff, tests Go y frontend, typecheck, build e imagen. Solo publica GHCR tras pasar. Publicación de imagen y despliegue son pasos separados; despliegue remoto vía SSH a entorno GitHub protegido con secrets, host key fijada e imagen por digest. Se comprueba readiness y se conserva el digest previo para rollback. No se hace push, se crean secrets ni se toca Azure sin datos y autorización de despliegue concretos.

No se presupone acceso actual al VPS, su distribución, arquitectura, dominios o URL de Prometheus. Son valores de configuración, no motivos para inventar una integración. La guía debe pedirlos al instalar y verificar prerrequisitos. Las dependencias externas se prueban mediante adaptadores y un entorno desechable antes de habilitar operaciones reales.

## Verificación y aceptación

1. Login real, sesión invalidable, CSRF y rechazo de accesos sin autorización; ausencia de credenciales predeterminadas.
2. Migraciones y consultas sqlc reproducibles con SQLite real en tests. Generación sin cambios pendientes.
3. Componentes shadcn reales, typecheck y build de producción. Verificación visual de login, dashboard, tablas, formularios y diálogos a 375, 768, 1024 y 1440 px; navegación por teclado y contraste mínimo 4.5:1 para texto normal.
4. Contratos de API y agente probados para IDs ajenos, argumentos maliciosos, socket no autorizado, errores Docker y logs limitados. Auditoría con secretos omitidos.
5. Instalación, DNS pendiente, fallo de Certbot, `nginx -t` inválido y rollback probados con ejecutores controlados; flujo real en host desechable antes de producción.
6. Backups con mantenimiento y recuperación tras error; restauración de staging, archivos maliciosos, espacio insuficiente y rollback probados sin usar datos del VPS de producción.
7. Reinicio de API o agente durante un trabajo: reconciliación por ID y sin repetir una acción incierta.
8. Sin integración disponible, los módulos muestran su estado y no simulan resultados. La vista previa de diseño permanece rotulada como mockup.
9. Compose válido; frontend servido con rutas SPA; acceso únicamente por Nginx; datos conservados tras recrear la API.

## Vistas previas y revisión del diseño

- [Dashboard: PNG](../../design/dashboard-preview.png) y [SVG editable](../../design/dashboard-preview.svg).
- [Login: PNG](../../design/login-preview.png) y [SVG editable](../../design/login-preview.svg).

Son vistas estáticas con datos ilustrativos, sin conexión al VPS. Definen composición e identidad; los controles funcionales y los estados responsivos corresponden a la implementación. Los controles e iconos finales se harán con shadcn/ui y Lucide.

Revisión independiente del documento: no se encontraron decisiones importantes pendientes para poder planificar. Los pares de texto de la paleta inicial se midieron con ratios de 5.80:1 a 15.18:1; los bordes de controles sobre blanco, 3.59:1. Esto comprueba esos pares concretos, no acredita todavía una UI implementada ni todos sus estados.

## Decisiones propuestas por el implementador

Son propuestas, no requisitos adicionales atribuidos al usuario: una sola máquina inicialmente; todos los usuarios como administradores; temas claro/oscuro no incluidos como selector; MariaDB por sitio; backups manuales locales; polling con pausa; sin carga de backups externos; journal local del agente y socket Unix como transporte. El usuario puede corregirlas al aprobar el diseño.

El plan decidirá las firmas de Go/TypeScript, esquema SQL detallado, versiones de dependencias compatibles, parámetros de Compose y casos concretos de tests. No podrá quitar ninguno de los siete módulos ni sustituirlos por pantallas decorativas.

## Referencias verificadas

- [Chi v5](https://github.com/go-chi/chi): routing y compatibilidad con `net/http`.
- [sqlc con SQLite](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html): generación con `modernc.org/sqlite` y `database/sql`.
- [Docker Engine SDK](https://docs.docker.com/reference/api/engine/sdk/): SDK Go y versión del API.
- [Protección del socket Docker](https://docs.docker.com/engine/security/protect-access/): el acceso al daemon requiere una frontera de confianza explícita.
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/): consultas instantáneas y por rango.
- [Certbot](https://eff-certbot.readthedocs.io/en/stable/using.html): emisión, renovación y hooks.
- [OWASP: contraseñas](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) y [sesiones](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html): base de las decisiones de autenticación.
- Skill local [UI/UX Pro Max](/home/devjair06/tlya_transportista/.agents/skills/ui-ux-pro-max/SKILL.md): minimalismo, tipografía de datos, componentes shadcn, diálogos y gráficas.
