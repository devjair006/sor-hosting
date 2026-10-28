# SOR Hosting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar el panel SOR Hosting completo para una VPS de Azure, con autenticación real, sitios WordPress, operaciones Docker, dominios y SSL, instalación, métricas y backups restaurables.

**Architecture:** La API Go/Chi sirve el frontend compilado y persiste en SQLite. Un agente Go nativo recibe operaciones tipadas por socket Unix, conserva su journal y controla Docker, Nginx y Certbot. Los trabajos son durables, auditados e idempotentes; el navegador nunca accede al agente ni al daemon Docker.

**Tech Stack:** Go, Chi v5, modernc.org/sqlite, sqlc, Argon2id; React, TypeScript, Vite, Tailwind CSS, shadcn/ui, React Router, TanStack Query, Lucide y Recharts; Docker Engine Go SDK, Prometheus, Nginx, Certbot, Compose, GHCR y GitHub Actions.

**Spec:** [Diseño aprobado](../specs/2026-10-27-sor-hosting-design.md). [Sistema visual aprobado](../../../design-system/sor-hosting/MASTER.md). Ambos son vinculantes.

## Global Constraints

- «La carpeta es `sor-hosting/`.» Marca visible: SOR Hosting; toda la interfaz en español.
- «Primera versión: un VPS y acceso de administradores, con login, dashboard, sitios WordPress, Docker Manager, dominios y SSL, instalador WordPress y backups con restauración.»
- «Nada de degradados: tampoco en gráficas, ilustraciones, estados de carga, botones o logotipo.»
- «La selección evita montar el socket Docker o las carpetas de Nginx y Certbot en el contenedor de la API.»
- «El agente verifica el UID/GID del cliente mediante credenciales del socket Unix.»
- «No ofrece shell, comandos arbitrarios ni proxy genérico del Docker API.»
- «Una integración ausente muestra «Sin conexión» o «Sin datos», nunca métricas inventadas.»
- «Sin registro público.» «Administradores con igual alcance en la primera versión.»
- «Cookie de producción `__Host-sor_session`, `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, sin `Domain`.»
- «caducidad por inactividad de 30 minutos y absoluta de 12 horas.» Tokens de sesión de 32 bytes, almacenados solo como hash.
- «Argon2id: sal aleatoria por contraseña, formato PHC y parámetros iniciales de 64 MiB, 3 iteraciones y paralelismo 1». Contraseñas de 12–128 caracteres.
- «Estados: `queued`, `running`, `succeeded`, `failed`, `interrupted`.» Mutaciones de infraestructura devuelven `202` e ID durable.
- «Destino inicial: directorio local fuera del webroot, `/var/lib/sor-hosting/backups`, con permisos restringidos y ruta configurable solo al instalar.» Backups manuales, sin Azure Blob ni carga de archivos externos en esta versión.
- «Confirmación escribiendo el dominio, comprobación previa y snapshot de seguridad obligatorio; si falla el snapshot no se continúa.»
- «No se hace push, se crean secrets ni se toca Azure sin datos y autorización de despliegue concretos.»
- La API escucha en `127.0.0.1` mediante Compose; agente en `/run/sor-hosting/agent.sock`; journal en `/var/lib/sor-hosting/agent/`.
- Frontend: Fira Sans + Fira Code locales; sidebar 248 px; header 72 px; marfil `#F7F8F2`, grafito `#15251E`, esmeralda `#08734F`; contraste y controles definidos en MASTER.
- Go compila con `CGO_ENABLED=0`. Consultas de producción tipadas con sqlc; no sustituir por un ORM.

## Review Focus

1. Pérdida de respuesta después de ejecutar una operación: no repetir una acción con efectos; probar reconciliación en tarea 4.
2. Caída del proceso durante mantenimiento: recuperar servicios y conservar rollback antes de aceptar nuevas operaciones; probar journal y recuperación en tareas 10 y 11.
3. DNS mixto, rebinding o redirección de un sitio hacia una IP interna: comprobar solo destinos públicos autorizados; probar tareas 7 y 8.
4. Dos sitios con nombres parecidos, IDs manipulados o contenedores reemplazados: no autorizar por nombre ni operar sobre infraestructura protegida; probar tareas 3 y 6.
5. Cambio de sesión en otra pestaña y logout con consultas en vuelo: invalidar datos privados y prevenir su reaparición; probar tarea 5.

## Orden y entregables

Es un producto integrado: los módulos comparten inventario, autorización, jobs y protocolo; se mantiene un plan secuencial, no proyectos independientes. Cada tarea termina en un componente comprobable. Dependencias: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12. Tareas 7 y 8 usan el inventario de 6; 9 usa 8; 10 y 11 usan la plantilla de 9.

La implementación recomendada es directa en esta sesión y con revisión independiente al final: conserva los contratos compartidos y evita reconstruir contexto por cada tarea. El usuario puede elegir subagentes y revisión por tarea antes de ejecutar.

## Mapa de archivos y contratos

Además de la estructura aprobada, agregar `shared/` para un único contrato API-agente, con su propio `go.mod`, y un `go.work` para `backend`, `agent` y `shared`. Módulos locales: `sor.hosting/backend`, `sor.hosting/agent` y `sor.hosting/shared`; usar reemplazos locales explícitos para compilación fuera de workspace. El módulo compartido no depende del backend ni del agente.

Los entrypoints configuran dependencias y shutdown; la lógica reside en paquetes pequeños por responsabilidad. Handlers y tests viven junto a su dominio. `frontend/src/services/types.ts` refleja el contrato HTTP documentado en `docs/api/openapi.yaml`.

### Tipos HTTP comunes

- Timestamps RFC3339 en UTC; números de métricas finitos; IDs aleatorios opacos; dominios normalizados ASCII/IDNA; secretos nunca en responses.
- `APIError = {error: {code: string, message: string, request_id: string}}`.
- `Page<T> = {items: T[], next_cursor: string | null}`; `limit` por defecto 25, máximo 100; cursor validado.
- `User = {id: string, email: string, role: "admin"}`.
- `Job = {id, kind, resource_id, state, phase, created_at, updated_at, error_code: string|null, message: string|null}`.
- Infraestructura: `POST` exige `Idempotency-Key`; devuelve `202 {job: Job}`. Igual clave, usuario, tipo y cuerpo devuelve el job original; cuerpo distinto devuelve `409 IDEMPOTENCY_CONFLICT`.
- `401 UNAUTHENTICATED`, `403 FORBIDDEN`, `404 NOT_FOUND`, `409 RESOURCE_BUSY`, `422 VALIDATION_ERROR`, `503 INTEGRATION_UNAVAILABLE`; errores de validación agregan `fields: Record<string,string>` al objeto `error`.
- Toda acción inicia un evento de auditoría. Si no se puede persistir el job y su auditoría, no se llama al agente.

### Contrato compartido del agente

`shared/agentapi/types.go` define enums y estructuras JSON, sin lógica de infraestructura. `shared/agentapi/client.go` define:

```go
type Client interface {
    Status(context.Context) (Status, error)
    Containers(context.Context) ([]Container, error)
    Logs(context.Context, LogsRequest) (LogsResult, error)
    Submit(context.Context, OperationRequest) (Operation, error)
    Operation(context.Context, string) (Operation, error)
    Download(context.Context, string) (io.ReadCloser, BackupFile, error)
}
```

`OperationRequest` tiene `ID`, `Kind`, `ServerID`, `ResourceID` y un payload JSON validado contra un tipo por kind: `container.start`, `container.stop`, `container.restart`, `site.adopt`, `site.install`, `domain.configure`, `certificate.issue`, `certificate.renew`, `backup.create`, `backup.restore`. Rechazar kinds/campos desconocidos. `Operation` contiene ID, estado, fase, error saneado y referencias a recursos creados. Operaciones y resultados se reconcilian por el mismo ID; logs de ejecución y secretos no forman parte del resultado.

El agente conserva un inventario local autorizado y un journal SQLite independientes del panel. API y agente validan límites y asociaciones; ninguna validación exclusiva del frontend permite saltárselos.

---

### Task 1: Base ejecutable, configuración y SQLite

**Files:** crear `.gitignore`, `go.work`, `Makefile`, `tools/bootstrap.sh`, `backend/go.mod`, `agent/go.mod`, `shared/go.mod`, `backend/cmd/server/main.go`, `backend/internal/config/config.go`, `backend/internal/database/{database.go,migrate.go}`, `backend/migrations/000001_initial.sql`, `backend/queries/{users,sessions,servers,sites,jobs,audit,domains,certificates,backups}.sql`, `backend/sqlc.yaml`, `docs/api/openapi.yaml`. Tests: `backend/internal/config/config_test.go`, `backend/internal/database/database_test.go`.

**Interfaces:** produce `config.Load() (Config,error)`, `database.Open(ctx context.Context, path string) (*sql.DB,error)` y `database.Migrate(ctx context.Context, db *sql.DB) error`; paquete generado `database/sqlc` con `New(*sql.DB) *Queries`. Esquema: entidades del spec, FK activas, restricciones de estado y unicidad para dominio/servidor, slug/servidor y usuario/idempotencia.

- [ ] **Step 1:** inicializar repo nuevo en rama `feat/initial-platform`, sin tocar repos vecinos; preparar toolchain local en `.tools/` ignorado si Go/sqlc no están disponibles. Descargar solo distribuciones oficiales y comprobar checksum; fijar versiones compatibles en módulos y herramienta, sin instalar paquetes del sistema. Registrar comandos en README. Crear tests `TestLoadRejectsMissingProductionSecrets`, `TestMigrationsReopenExistingDatabase`, `TestForeignKeysEnforced`, `TestDuplicateDomainRejected`, `TestDatabaseBusyTimeout` con SQLite temporal real; aserciones `err != nil` para secretos vacíos y FK inválidas, `userCount == 1` tras reapertura.
- [ ] **Step 2:** ejecutar `cd backend && go test ./internal/config ./internal/database -count=1`; debe fallar por APIs aún inexistentes.
- [ ] **Step 3:** implementar configuración validada, migraciones embebidas con versión, transacción por migración, WAL/busy timeout y consultas sqlc. Estados de sitios: `provisioning`, `pending_dns`, `active`, `failed`, `restoring`. Arranque aplica migraciones o falla cerrado; servidor local inicial soporta `/healthz` y `/readyz` sin información sensible.
- [ ] **Step 4:** ejecutar `sqlc generate` en backend y repetir tests; compilar `CGO_ENABLED=0 go build ./cmd/server`. Resultado: tests verdes y binario sin CGO.
- [ ] **Step 5:** commit `feat: initialize SOR Hosting API and SQLite persistence`, incluidos documentos de diseño ya aprobados. Excluir `.tools`, `.env`, bases, backups y binarios.

### Task 2: Login, sesiones, CSRF y bootstrap

**Files:** crear `backend/internal/auth/{password.go,service.go,middleware.go,handlers.go,ratelimit.go,bootstrap.go}`, `backend/internal/httpapi/{router.go,response.go,server.go}`, `backend/internal/auth/{password_test.go,session_test.go,handlers_test.go}`; modificar `backend/cmd/server/main.go`, queries y OpenAPI.

**Interfaces:** consume Config/Queries de 1. Produce `auth.NewService(queries *sqlc.Queries, cfg Config, clock func() time.Time) *Service`, `(*Service).RequireSession(http.Handler) http.Handler`, `(*Service).RequireCSRF(http.Handler) http.Handler`, `auth.RegisterRoutes(chi.Router,*Service)` y `auth.CurrentUser(context.Context) (User,bool)`. OpenAPI: `POST /auth/login {email,password}`, `GET /auth/me → {user}`, `GET /auth/csrf → {csrf_token}`, `POST /auth/logout → 204`, `POST /auth/password {current_password,new_password} → 204`.

- [ ] **Step 1:** escribir `TestArgon2RoundTripAndMalformedPHC`, `TestLoginCookieFlags`, `TestSessionIdleAndAbsoluteExpiry`, `TestLogoutInvalidatesSession`, `TestPasswordChangeRevokesAllSessions`, `TestCSRFMissingOrCrossOrigin`, `TestLoginRateLimits`, `TestBootstrapHasNoDefaultUser`. Usar reloj inyectado: a `30m+1s` sin actividad y a `12h+1s` absolutos resulta `401`; verificar `HttpOnly && Secure && SameSiteStrict` y ausencia de `Domain`; nunca contraseña/token sin hash en DB.
- [ ] **Step 2:** `cd backend && go test ./internal/auth ./internal/httpapi -count=1`; esperar fallo funcional por handlers todavía ausentes.
- [ ] **Step 3:** implementar Argon2id/PHC con límites de parseo y concurrencia, tokens aleatorios, hash persistido, rotación, logout y cambio de contraseña. Bootstrap mediante stdin o secret-file; no argumentos con contraseña ni credenciales por defecto. Aplicar Origin/CSRF y límite de cuerpo; lista de proxies confiables configurable y rate limiter con expiración y tamaño acotado. Limitar intentos antes de ejecutar Argon2; mantener errores de login indistinguibles.
- [ ] **Step 4:** repetir tests y `go test ./... -count=1`. Comprobar que rutas privadas siempre pasan por autorización y que API desconocida devuelve JSON, no fallback SPA.
- [ ] **Step 5:** commit `feat: add administrator authentication and secure sessions`.

### Task 3: Protocolo, socket Unix y agente autorizado

**Files:** crear `shared/agentapi/{types.go,client.go,validation.go,validation_test.go}`, `backend/internal/agentclient/{client.go,client_test.go}`, `agent/cmd/agent/main.go`, `agent/internal/rpc/{server.go,peer_linux.go,server_test.go}`, `agent/internal/config/config.go`, `agent/internal/journal/{store.go,store_test.go}`, `agent/internal/inventory/{store.go,policy.go,policy_test.go}`, `agent/{sqlc.yaml,migrations/000001_agent.sql,queries/journal.sql,queries/inventory.sql}`.

**Interfaces:** produce Client y tipos compartidos descritos arriba; constructor `agentclient.New(socketPath string, timeout time.Duration) agentapi.Client`. Agente: `rpc.NewServer(cfg Config, handlers Handlers) (*http.Server,error)` sobre listener Unix con verificación `SO_PEERCRED`. `journal.Reserve(ctx, request) (operation, created, error)` es atómico por ID; inventario autoriza IDs Docker exactos, funciones del contenedor y operaciones permitidas.

- [ ] **Step 1:** escribir `TestUnknownOperationRejected`, `TestUnauthorizedPeerRejected`, `TestSocketPermissions`, `TestReserveIDReturnsSameOperation`, `TestIDWithDifferentPayloadRejected`, `TestForeignContainerRejected`, `TestProtectedContainerCannotStop` y `TestReplacementIDNotAuthorizedByName`. Aserciones: kind inválido `422`, peer ajeno `403`, socket `0660`, una reserva creada tras 20 envíos concurrentes, nunca autorización por nombre.
- [ ] **Step 2:** `cd shared && go test ./...`; `cd agent && go test ./internal/rpc ./internal/journal ./internal/inventory`; `cd backend && go test ./internal/agentclient`; deben fallar antes de la implementación.
- [ ] **Step 3:** implementar contrato, cliente HTTP sobre socket, controles de peer UID/GID, directorio `0750`, validación tipada y journal durable. No seguir ni borrar sockets de procesos activos al arrancar; rechazar configuraciones inseguras. El inventario protege IDs configurados y etiquetas de infraestructura; adoptar exige aprobación local por ID exacto en configuración, no solo una petición del navegador. Generar consultas sqlc del agente.
- [ ] **Step 4:** repetir tests de los tres módulos y compilar el agente Linux con `CGO_ENABLED=0`. Probar autorización sobre un socket temporal real, usando proveedor de credenciales inyectado para casos denegados.
- [ ] **Step 5:** commit `feat: add restricted Unix agent and durable operation journal`.

### Task 4: Jobs, auditoría y recuperación

**Files:** crear `backend/internal/jobs/{service.go,worker.go,reconcile.go,secrets.go,worker_test.go}`, `backend/internal/audit/{service.go,handlers.go,service_test.go}`, `agent/internal/operations/{dispatcher.go,recovery.go,dispatcher_test.go}`; modificar journal/queries, router y OpenAPI.

**Interfaces:** consume agentapi.Client y Queries. Produce `jobs.Enqueue(ctx context.Context, actorID, key string, request agentapi.OperationRequest) (Job,error)`, `jobs.Run(ctx context.Context) error`, `audit.Record(ctx context.Context,event Event) error`. UI consume `GET /jobs/{id} → {job}` y `GET /audit → Page<AuditEvent>`; cursor estable por timestamp/ID. Resultado sanitizado distingue error de integración y rechazo de política.

- [ ] **Step 1:** escribir `TestEnqueueAuditsAtomically`, `TestIdempotencyConflict`, `TestLostAgentResponseDoesNotRepeatAction`, `TestRestartReconcilesCompletedAgentJob`, `TestUncertainOperationBecomesInterrupted`, `TestConcurrentSiteJobsConflict`, `TestSecretsEncryptedAndRemovedOnTerminalState`, `TestAuditRedactsSecrets`. Aserciones: fake executor llamado exactamente una vez aunque la primera respuesta se pierda; un job incierto termina `interrupted`, no `succeeded`; auditoría sin password/cookie/token.
- [ ] **Step 2:** `cd backend && go test ./internal/jobs ./internal/audit -count=1`; `cd agent && go test ./internal/operations -count=1`; esperar fallo.
- [ ] **Step 3:** implementar transacciones, claves de idempotencia ligadas a actor/cuerpo, compare-and-swap de estado, AEAD con clave externa de 32 bytes, límites de cola y reconciliación al iniciar. Agent dispatcher registra antes de ejecutar, serializa por sitio y por infraestructura global, y conserva resultados. No reejecutar automáticamente una fase iniciada sin resultado; reconciliar observación o exigir reintento seguro con nuevo job.
- [ ] **Step 4:** repetir pruebas y simular reinicio entre reserva, ejecución y recepción con stores SQLite persistentes temporales. GET de jobs/auditoría debe responder `401` sin sesión.
- [ ] **Step 5:** commit `feat: add auditable jobs with restart reconciliation`.

### Task 5: Frontend, identidad, login y navegación

**Files:** crear `frontend/{package.json,package-lock.json,tsconfig.json,vite.config.ts,index.html,components.json}`, `frontend/src/{main.tsx,App.tsx,index.css}`, `frontend/src/components/{ui/,layout/}`, `frontend/src/pages/{LoginPage.tsx,DashboardPage.tsx}`, `frontend/src/hooks/{useSession.ts,useJob.ts}`, `frontend/src/services/{api.ts,types.ts}`, `frontend/src/lib/queryClient.ts`, `frontend/src/assets/{brand.svg,fonts/}` y `frontend/src/test/{setup.ts,auth.test.tsx}`; incluir Vitest, Testing Library y MSW solo como dev dependencies.

**Interfaces:** consume auth/jobs de 2 y 4. Produce `api<T>(path: string, options?: RequestInit): Promise<T>`, `useSession(): {user: User|null, status, login, logout}`, `useJob(id: string|null)`; exportar layout `AppShell` y primitivas shadcn adaptadas. React Router: `/login`, `/`, `/sites`, `/sites/new`, `/sites/:id`, `/containers`, `/domains`, `/backups`, `/audit`, `/settings`.

- [ ] **Step 1:** escribir `login succeeds and returns to protected route`, `401 clears session and cached private queries`, `logout ignores late private responses`, `cross-tab logout hides private data` y `login error is announced and preserves email`. Aserciones: navegación al resumen solo tras sesión confirmada; tras logout no vuelve a aparecer el dominio privado por una respuesta tardía; contraseña permite pegar, tiene label y botón accesible.
- [ ] **Step 2:** `cd frontend && npm test -- --run`; las pruebas deben fallar sin proveedor, rutas y formulario implementados.
- [ ] **Step 3:** instalar versiones compatibles y fijar lockfile; agregar componentes reales con CLI oficial de shadcn y personalizarlos con MASTER. Implementar sesión/CSRF por cookie, cancelación de requests y limpieza de QueryClient por sesión, propagación de logout entre pestañas, sidebar responsiva y fuentes locales. Hacer login y AppShell fieles a los mockups. Las páginas aún no integradas muestran estado explícito de integración, nunca acciones de éxito falsas.
- [ ] **Step 4:** `npm test -- --run`, `npm run typecheck`, `npm run build`. Verificar login real con API de 2, acceso directo a ruta protegida, teclado, móvil y ausencia de degradados. Cada módulo se completa en sus tareas siguientes; no considerar esta tarea como el frontend terminado.
- [ ] **Step 5:** commit `feat: implement SOR Hosting identity and authenticated app shell`.

### Task 6: Inventario, Docker Manager y adopción de WordPress

**Files:** crear `agent/internal/docker/{client.go,containers.go,logs.go,containers_test.go}`, `agent/internal/wordpress/{adopt.go,adopt_test.go}`, `backend/internal/containers/{service.go,handlers.go,handlers_test.go}`, `backend/internal/sites/{service.go,handlers.go,handlers_test.go}`, `frontend/src/pages/{ContainersPage.tsx,SitesPage.tsx,SiteDetailPage.tsx}`, `frontend/src/components/containers/{ContainerActions.tsx,LogsPanel.tsx}`, `frontend/src/components/sites/AdoptSiteDialog.tsx`, tests de interacción junto a componentes; actualizar shared, OpenAPI y queries.

**Interfaces:** consume Client/jobs/policy. `GET /containers → Page<Container>` incluye `managed`, `can_manage`, `can_read_logs`, `protected` y motivo; `GET /containers/{id}/logs?tail=200 → {lines,truncated}`; `POST /containers/{id}/actions {action:"start"|"stop"|"restart"}`. `GET /sites → Page<Site>`, `GET /sites/{id} → {site,containers,domains,backups}`, `POST /sites/adopt {name,domain,web_container_id,database_container_id,writer_container_ids,database_secret_ref}`. Tipos y filtros `q`, `status` se fijan en OpenAPI; referencias secretas se configuran localmente.

- [ ] **Step 1:** escribir `TestContainerActionRequiresBothRegistries`, `TestContainerStatsZeroCPUIsNotNaN`, `TestLogsLimitedAndKnownSecretsRedacted`, `TestForeignSiteNotFound`, `TestAdoptionDoesNotRestartResources`, `TestAdoptionRejectsUnapprovedLocalIDs`; UI `protected container has no stop action` y `restart displays job progress, not immediate success`. Esperar `403` sin permiso, `tail > 1000 → 422`, salida máxima 256 KiB, logs de ID extranjero denegados.
- [ ] **Step 2:** ejecutar pruebas de paquetes nuevos en agente/backend y pruebas frontend focalizadas; confirmar fallos esperados.
- [ ] **Step 3:** implementar adaptador Docker Engine Go SDK con negociación API, timeouts y demultiplexado de logs. Incorporar asociación exacta de recursos, adopción de plantilla compatible sin modificar contenedores y métricas de contenedor donde Docker las ofrezca. Frontend con filtros, tabla/detalle, menús autorizados y logs pausados; enlace `/wp-admin` normal, sin autenticación automática.
- [ ] **Step 4:** repetir pruebas, verificar errores del daemon y desconexión sin resultados simulados. Integración real de start/stop/restart/logs solo sobre contenedores desechables cuando exista daemon autorizado; registrar claramente si no está disponible.
- [ ] **Step 5:** commit `feat: manage authorized containers and WordPress inventory`.

### Task 7: Prometheus, dashboard y salud de sitios

**Files:** crear `backend/internal/monitoring/{prometheus.go,queries.go,health.go,handlers.go,prometheus_test.go,health_test.go}`, `backend/internal/monitoring/dashboard.go`, `frontend/src/components/monitoring/{MetricCard.tsx,ResourceChart.tsx,ServiceHealth.tsx}`, completar `DashboardPage.tsx`, tests de chart; actualizar OpenAPI.

**Interfaces:** `monitoring.NewClient(baseURL string, instance string, transport http.RoundTripper) *Client`; `GET /dashboard → {server,metrics,services,sites,recent_activity,observed_at}`, `GET /monitoring?range=1h|6h|24h → {series,observed_at,stale}`. Series con `{name,unit,points:[{timestamp,value}]}`; métrica ausente `null`, nunca cero de reemplazo.

- [ ] **Step 1:** escribir `TestPrometheusQueriesSelectConfiguredInstance`, `TestMetricMissingIsNull`, `TestNaNInfinityAndStaleSamples`, `TestHealthProbeBlocksPrivateAndRedirectedTargets`, `TestHealthDNSRebindingPinnedDial`, y UI `pause stops refresh and exposes last sample time`. Aserciones: `NaN/+Inf/-Inf` no se serializan como números; Prometheus `up=1` no marca WordPress sano; dominios ajenos no son destinos de probes.
- [ ] **Step 2:** ejecutar tests de monitoring y chart; esperar fallos antes de implementación.
- [ ] **Step 3:** implementar query/query_range acotados, selector escapado, máximo 600 puntos por serie y timeouts. Rangos 1/6/24 horas, refresco de 30 s; tasa CPU/tráfico, RAM disponible y filesystem principal. Probes fijan IP pública validada durante la conexión y revalidan redirecciones; no alcanzan metadata de Azure ni redes internas. Completar dashboard fiel al mockup y gráficas con líneas sólidas, unidades, leyendas, pausa y resumen textual.
- [ ] **Step 4:** correr tests y build frontend; probar respuestas reales con fixture HTTP de Prometheus, errores y series ausentes. Verificar gráfico por teclado y estado sin conexión.
- [ ] **Step 5:** commit `feat: add Prometheus monitoring and server dashboard`.

### Task 8: Dominios, Nginx y SSL

**Files:** crear `shared/domainname/{normalize.go,normalize_test.go}`, `agent/internal/nginx/{manager.go,templates.go,manager_test.go}`, `agent/internal/certbot/{client.go,client_test.go}`, `agent/internal/executil/{runner.go,runner_test.go}`, `backend/internal/domains/{service.go,handlers.go,service_test.go}`, `backend/internal/ssl/{service.go,handlers.go,service_test.go}`, `frontend/src/pages/DomainsPage.tsx`, `frontend/src/components/domains/{DomainDialog.tsx,CertificateDetails.tsx}`, tests y OpenAPI.

**Interfaces:** produce `domainname.Normalize(value string) (string,error)`, `executil.Runner.Run(ctx context.Context, executable string, args []string, stdin io.Reader) (Result,error)`. Endpoints: `POST /domains {site_id,hostname}`, `GET /domains`, `POST /domains/{id}/check`, `POST /certificates/{id}/renew`; todas las escrituras usan jobs. `nginx.Manager.Apply(ctx, siteID, hostname string, upstreamPort uint16) error`; `certbot.Client.Issue/Renew(ctx, certificateSpec) (Certificate,error)`.

- [ ] **Step 1:** escribir `TestDomainRejectsConfigInjection`, `TestIDNNormalizationAndUniqueness`, `TestMixedAAndAAAARequiresMatchingPublicHost`, `TestNginxValidationFailureRestoresPreviousConfig`, `TestNginxSymlinkEscapeRejected`, `TestCertbotRenewSelectsRegisteredCertName`, `TestCertificateNotDueDoesNotForceRenewal`. Verificar argumentos sin shell, exactamente el cert-name conocido y ausencia de `--force-renewal`.
- [ ] **Step 2:** tests de dominios/shared, nginx/certbot y handlers deben fallar antes del código productivo.
- [ ] **Step 3:** implementar operaciones serializadas, staging de Nginx con prueba de configuración completa y rollback, archivos solo bajo directorio SOR y executables allowlisted; Certbot HTTP-01 con webroot. Persistir fingerprint, nombres y expiración sin copiar clave privada. UI separa DNS, HTTP, certificado y HTTPS; explicación del registro DNS externo. Renovación usa certificado registrado y hook idempotente, no modifica timers existentes.
- [ ] **Step 4:** repetir tests; comprobar combinaciones de DNS, proceso fallido y timeout con runner inyectado. Validación real `nginx -t`/ACME staging solo en host desechable, nunca editar el VPS actual en tests.
- [ ] **Step 5:** commit `feat: configure managed domains and certificate lifecycle`.

### Task 9: Instalador WordPress completo

**Files:** crear `agent/internal/wordpress/{install.go,resources.go,secrets.go,install_test.go}`, `backend/internal/sites/{install.go,install_test.go}`, `frontend/src/pages/CreateSitePage.tsx`, `frontend/src/components/sites/{InstallSteps.tsx,InstallSummary.tsx}`, `frontend/src/test/install.test.tsx`; actualizar inventario, jobs, schemas y OpenAPI.

**Interfaces:** `POST /sites {name,domain,admin_email,admin_password,wordpress_title,resources:{memory_mb,cpu_limit}} → {job}`. `InstallSiteInput` compartido conserva IDs y secretos cifrados solo donde se requieren. `wordpress.Installer.Install(ctx, request, checkpoint) (SiteResources,error)`; `SiteResources` incluye IDs exactos, volúmenes, red, puerto loopback y secret refs, nunca plaintext.

- [ ] **Step 1:** escribir `TestInstallOrderWaitsForDatabase`, `TestDatabaseIsNotPublished`, `TestWebPortIsLoopbackOnly`, `TestSecretsDoNotAppearInCommandArgsOrResult`, `TestDNSPendingPreservesProvisionedSite`, `TestFailureCleanupOnlyTouchesCurrentJob`, `TestConcurrentInstallsReserveUniquePorts`; UI `wizard preserves values when going back` y `pending DNS is not installation failure`.
- [ ] **Step 2:** ejecutar tests agent/wordpress, backend/sites y frontend/install; comprobar fallo previo.
- [ ] **Step 3:** fijar imágenes oficiales WordPress/MariaDB/WP-CLI compatibles y digests comprobados; crear red y volúmenes por sitio con labels de sitio/job. Usar env-files/secret-files `0600`; no contraseñas en argv, logs ni respuestas. WP-CLI recibe secretos mediante entrada protegida, con wrapper fijo sin interpolación de usuario. Escribir checkpoint antes de cada efecto; readiness con timeout; completar Nginx, luego SSL si DNS corresponde. Limpiar únicamente recursos nuevos registrados para ese ID cuando sea seguro. Asistente de cuatro pasos con revisión y progreso durable.
- [ ] **Step 4:** repetir suites focalizadas y prueba de integración sobre un proyecto Docker desechable: sitio sirve, wp-admin accesible por enlace, DB no publicada, fallo DNS permite reintento de SSL. Si el entorno no ofrece Docker, mantener esa prueba marcada como no ejecutada y exigirla antes de producción.
- [ ] **Step 5:** commit `feat: provision WordPress sites with isolated database and volumes`.

### Task 10: Backups consistentes y descarga autorizada

**Files:** crear `agent/internal/backups/{manifest.go,create.go,archive.go,recovery.go,create_test.go}`, `backend/internal/backups/{service.go,handlers.go,download.go,handlers_test.go}`, `frontend/src/pages/BackupsPage.tsx`, `frontend/src/components/backups/CreateBackupDialog.tsx`, tests y OpenAPI.

**Interfaces:** `POST /sites/{id}/backups {confirm_maintenance:true} → {job}`, `GET /backups → Page<Backup>`, `GET /backups/{id}/download` transmite archivo solo a administrador autorizado. `backups.Create(ctx context.Context, site SiteResources, operationID string) (Manifest,error)`. Manifest versión 1 contiene sitio, UTC, engine/version, nombres de artefactos, tamaños, checksums SHA-256 y referencias de recursos sin contraseñas.

- [ ] **Step 1:** escribir `TestBackupStopsAllDeclaredWriters`, `TestBackupRestartsServicesOnDumpFailure`, `TestBackupPreservesInitiallyStoppedServices`, `TestBackupRecoveryAfterProcessCrash`, `TestBackupRejectsUnknownWriters`, `TestBackupDownloadRejectsUnregisteredID`, `TestBackupFilesNeverReachPublicStaticServer`. Aserciones: solo arrancan servicios que estaban activos, una descarga ajena `404`, fallo de dump no registra backup exitoso.
- [ ] **Step 2:** ejecutar tests nuevos de agente/backend y flujo UI; esperar fallo antes del código.
- [ ] **Step 3:** implementar journal de mantenimiento con estado previo de servicios, parada/graceful drain de escritores, dump lógico, archivo y manifest con checksum; terminar con rename atómico. Recuperación de servicios tanto por error controlado como por reinicio del agente; detener nuevos jobs para el sitio hasta recuperar su estado. Volumen local fuera de webroot y rutas derivadas exclusivamente del inventario. UI explica mantenimiento, destino local y limitación frente a pérdida de VPS.
- [ ] **Step 4:** repetir tests y verificar que el backup incluye DB/archivos recuperables, no cookies ni secretos del panel; descarga streaming acotada sin cargar archivo completo en memoria. Integración real solo con sitio temporal de 9.
- [ ] **Step 5:** commit `feat: add consistent WordPress backups and private downloads`.

### Task 11: Restauración con staging y rollback

**Files:** crear `agent/internal/backups/{validate.go,restore.go,rollback.go,validate_test.go,restore_test.go}`, `backend/internal/backups/{restore.go,restore_test.go}`, `frontend/src/components/backups/RestoreDialog.tsx`, `frontend/src/test/restore.test.tsx`; actualizar journal y OpenAPI.

**Interfaces:** `POST /backups/{id}/restore {confirm_domain,confirm_maintenance:true} → {job}`; `backups.Restore(ctx context.Context, registeredBackupID string, confirmedDomain string, operationID string) (RestoreResult,error)`. Usa backup.create de 10 para snapshot obligatorio y nginx.Manager de 8 para intercambio/rollback. El estado operativo de contenedores activos queda versionado en inventario, no solo en memoria.

- [ ] **Step 1:** escribir `TestRestoreRejectsTraversalAbsolutePathsSymlinksAndOversize`, `TestRestoreChecksDigestAndAvailableSpace`, `TestRestoreRequiresSameSiteAndDomainConfirmation`, `TestFailedSafetyBackupDoesNotTouchSite`, `TestStagingHealthFailureKeepsOriginalUpstream`, `TestFailedProxySwitchRollsBack`, `TestCrashDuringCutoverRecoversFromJournal`. Aserciones: snapshot fallido → cero cambios de upstream; checksum inválido → cero extracción; tras fallo de cutover existe al menos el destino previo recuperable.
- [ ] **Step 2:** ejecutar suites de restore, handlers y diálogo; confirmar fallo funcional previo.
- [ ] **Step 3:** validar archivo completo antes de extraer, rechazar rutas inseguras/enlaces y aplicar límites de espacio/tamaño. Crear volúmenes/DB/contenedores de staging, importar dump y archivos, comprobar WordPress y cambiar Nginx bajo bloqueo. Conservar manifest de rollback y recursos anteriores hasta verificar; nunca eliminar el único estado recuperable. Al iniciar, reconciliar fase de cutover sin repetir importación incierta. El diálogo exige dominio escrito y muestra progreso, resultado y rollback.
- [ ] **Step 4:** repetir tests; round-trip real en sitio desechable: crear contenido, backup, modificar contenido, restaurar y verificar contenido/DB y URL original. Guardar resultado; sin daemon autorizado no declarar completada esta integración.
- [ ] **Step 5:** commit `feat: restore WordPress backups with validated staging and rollback`.

### Task 12: Auditoría UI, configuración, producción y verificación integral

**Files:** crear `frontend/src/pages/{AuditPage.tsx,SettingsPage.tsx}`, `frontend/e2e/{auth.spec.ts,sites.spec.ts,operations.spec.ts,accessibility.spec.ts}`, `frontend/playwright.config.ts`, `backend/internal/httpapi/{static.go,static_test.go}`, `deploy/{Dockerfile,compose.yaml,.env.example}`, `deploy/nginx/{panel.conf,sor-sites-include.conf,certbot-deploy-hook.sh}`, `deploy/systemd/sor-hosting-agent.service`, `deploy/{install-agent.sh,release.sh}`, `.github/workflows/deploy.yml`, `README.md`, `docs/{installation.md,operations.md,testing.md}`; tests de config/HTTP y scripts verificables sin mutar host.

**Interfaces:** consume endpoints y primitivas anteriores. `GET /settings → {server,integrations,backup_destination}`, `PATCH /settings {server_name} → {server}`; no acepta URLs/rutas/secretos arbitrarios. SPA fallback solo para rutas no API y archivos de producción. CLI documentado: bootstrap-admin, migrate, serve; agente con config-file sin secretos en argv.

- [ ] **Step 1:** escribir `TestSPAFallbackDoesNotMaskMissingAPI`, `TestStaticPathTraversalRejected`, `TestReadinessFailsWithoutDatabase`, `TestSettingsRejectsIntegrationURLs`, pruebas E2E login/logout, navegación directa, acciones con jobs, restore con confirmación y errores de integración. Capturas a 375/768/1024/1440 px; axe o comprobación equivalente para login, navegación, diálogos y formularios, complementada con teclado y contraste manual.
- [ ] **Step 2:** ejecutar los tests antes de implementar handlers/static/configuración faltantes y verificar el fallo esperado. Los E2E de infraestructura usan adaptadores controlados explícitos; no se presenta ese entorno como una operación real del VPS.
- [ ] **Step 3:** completar Auditoría/Configuración y servir build Vite desde API. Crear Dockerfile multi-stage con Go `CGO_ENABLED=0` y runtime sin root; Compose publica solo loopback, monta datos y socket del agente, nunca docker.sock ni carpetas de Nginx/Certbot. Instalación del agente idempotente y con dry-run; systemd aplica límites compatibles con sus tareas, sin aparentar que opera sin privilegios. Documentar uid/gid y socket, secretos, include de Nginx con copia/validación y timer Certbot existente.
- [ ] **Step 4:** crear workflow con jobs separados de test/build, publicación GHCR y despliegue por `workflow_dispatch` a entorno protegido; pin de acciones por SHA verificado al implementar, SSH con known_hosts fijado, permisos mínimos, despliegue por digest, readiness y rollback de imagen. `release.sh` conserva backup SQLite coherente antes de migrar; rollback de imagen solo si esquema compatible, nunca copiar un `.db` abierto sin checkpoint/backup API ni degradar esquema automáticamente. Documentar recuperación y el punto de intervención manual si una migración no es reversible.
- [ ] **Step 5:** ejecutar `go test ./... -count=1` en los tres módulos, `go vet ./...`, generación sqlc sin diff, build de backend y agente con CGO desactivado; frontend `npm ci`, tests, typecheck, build y Playwright. Ejecutar `docker compose -f deploy/compose.yaml config --quiet` con variables de prueba no secretas. Construir imagen y smoke test únicamente si el daemon está autorizado; listar cualquier comprobación pendiente por el entorno.
- [ ] **Step 6:** comparar UI con ambos mockups y MASTER; probar keyboard/zoom/reduced-motion, móvil, carga/vacío/error y cero degradados. Revisión de todos los endpoints contra autorización, CSRF, SSRF, IDs ajenos, redacción y límites. Revisión de cobertura: siete módulos funcionales, no páginas decorativas. Documentar prueba real completa en host desechable antes de habilitar producción.
- [ ] **Step 7:** commit `feat: package SOR Hosting for verified deployment`; solicitar una revisión independiente del resultado completo conforme a la skill de ejecución elegida. Corregir hallazgos importantes con test que reproduzca el problema y suite verde, sin publicar ni desplegar todavía. Entregar instrucciones de arranque y resultados concretos, indicando pruebas realmente ejecutadas y cualquier requisito externo pendiente.

## Comprobación del plan

- Alcance: login en 2/5; dashboard en 7; WordPress/inventario en 6; Docker en 3/6; dominios/SSL en 8; instalador en 9; backups/restauración en 10/11; auditoría/configuración/producción en 4/12.
- Los cinco riesgos de Review Focus tienen tests explícitos en sus tareas; los contratos comunes fijan respuestas, IDs, errores y jobs.
- Se conserva el mismo `agentapi.Client`, tipos compartidos e IDs entre API, worker y agente; frontend consume OpenAPI y types.ts. Los cambios de contrato actualizan ambos en la misma tarea.
- Los únicos datos externos necesarios para producción son configuración de VPS, dominios/DNS, Prometheus, paths autorizados, secrets GHCR/SSH y aprobación del despliegue. No impiden implementar ni probar localmente los adaptadores.
- Entorno observado al planificar: Node/npm disponibles; Go/sqlc no detectados en PATH ni rutas estándar revisadas; Docker CLI disponible pero daemon denegado. La tarea 1 prepara toolchain local cuando sea posible. Pruebas del host y contenedores reales requieren un entorno con acceso y se informan con precisión.
- Estado actual: diseño aprobado; este plan pendiente de revisión del usuario. No hay backend, agente ni frontend implementados todavía.

## Revisión y elección de ejecución

Antes de implementar, revisar este plan y elegir: **directa en esta sesión**, con revisión independiente final, o **con subagentes**, con revisión por tarea y revisión final. Recomendación del implementador: directa por los contratos compartidos entre API, agente y UI; conserva una revisión independiente antes de entregar.
