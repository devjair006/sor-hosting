# Docker Edge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox syntax for tracking.

**Goal:** Soportar Nginx/Certbot Docker y modo nativo con protección de infraestructura existente.
**Architecture:** Adaptadores sobre Moby y ejecutor nativo; rutas explícitas por namespace; transacciones Nginx existentes; red proxy externa para web.
**Tech Stack:** Go 1.26, Moby v0.6.2, SQLite/sqlc, Docker, systemd, GitHub Actions.
**Spec:** docs/superpowers/specs/2026-10-28-docker-edge-design.md

## Global Constraints
- No ejecutar cambios sobre el VPS real. No modificar producción automáticamente.
- No reemplazar default.conf ni modificar vhosts actuales.
- Conservar modo nativo, ProtectHome y aislamiento DB.
- Contenedores y certificados preexistentes protegidos; artefacto Linux amd64 con checksum.

## Review Focus
- Mount incorrecto o ruta de /home: rechazar antes de escribir.
- Contenedor parado/recreado o exec con exit != 0: estado real y rollback sin shell.
- Lineage ajena con prefijo sor-: rechazar sin autorización local del diario.
- Restauración en red proxy: cambiar upstream al staging y restaurar el anterior si falla.
- Cancelación Certbot y hooks heredados: cleanup exacto y sin modificar otras lineages.

### Task 1: Configuración y adaptadores Docker
**Files:** agent/internal/config/*, agent/internal/dockerexec/*, agent/internal/docker/client.go.
**Interfaces:** Config.Nginx/Certbot; dockerexec.Executor implementa executil.Executor y Alive/Mounts; RunCertbot contenedor temporal.
- [ ] Escribir tests para defaults/validación y exec/identidad/cancelación, verificar fallo.
- [ ] Implementar configuración y ejecución restringida con SDK, proteger nombre de Nginx.
- [ ] Ejecutar tests y commit.

### Task 2: Nginx/Certbot y upstream por red
**Files:** agent/internal/nginx/*, agent/internal/certbot/*, agent/internal/wordpress/*, agent/internal/backups/restore.go, agent/internal/rpc/*, agent/cmd/agent/main.go.
**Interfaces:** nginx.ApplyUpstream/Available, Certbot.Available/Managed, wordpress.ApplyProxy.
- [ ] Tests RED para rutas internas/host, conflicto de vhost, cert ajeno y rollback.
- [ ] Integrar adaptadores y estado real; web en proxy network, DB aislada; verificar cert path confinado.
- [ ] Tests GREEN y regresiones nativas; commit.

### Task 3: Integración, despliegue manual y artifact
**Files:** agent/internal/*/*integration_test.go, deploy/*, docs/docker-edge.md, tools/test-docker-edge.sh, .github/workflows/deploy.yml.
**Interfaces:** bash tools/test-docker-edge.sh; artifact sor-agent-linux-amd64 + SHA256SUMS.
- [ ] Crear fixture Docker Nginx/Certbot y tests de exec, status, mounts, errores y rollback; verificar RED/GREEN.
- [ ] Ejemplos de agent.json, mounts systemd y Compose override exclusivamente manual; documentar rollback y protección de vhosts/certs.
- [ ] Artifact amd64 sin CGO, tests/build completos, integración local y revisión independiente; commit.
