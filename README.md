# SOR Hosting

Panel privado para una VPS Linux en Azure. Go + Chi v5, React/TypeScript/Vite, Tailwind y shadcn/ui, SQLite/modernc/sqlc, sesiones con Argon2id, agente local y métricas Prometheus. Identidad grafito, marfil y esmeralda, sin degradados.

## Incluye

Login administrativo; dashboard de CPU/RAM/disco/tráfico; inventario WordPress y enlaces wp-admin; operaciones Docker autorizadas y logs limitados; dominios/SSL con Nginx y Certbot existentes; instalador WordPress/MariaDB aislado por sitio; backups manuales, descarga y restauración con staging/rollback; auditoría y configuración.

La API vive en Docker. El agente corre bajo systemd en el VPS, con acceso local al Docker Engine y los servicios de host. El contenedor de API solo recibe el socket del agente, nunca docker.sock. El frontend usa la API real: si una integración falta, muestra su indisponibilidad.

## Desarrollo

Requisitos: Go 1.26, Node 24 o superior, npm y acceso a los registros de dependencias. Docker Compose v2, Nginx y Certbot se necesitan para las integraciones del agente, no para compilar el frontend.

```bash
./tools/bootstrap.sh
make check
make build
```

`bootstrap.sh` instala sqlc en `.tools/bin`, resuelve los módulos, genera consultas tipadas y prepara npm. No instala paquetes del sistema. Si existe `.tools/go/bin/go`, tanto el script como `make` lo usan automáticamente. El SQL y los manifiestos son la fuente; los paquetes sqlc se generan antes de compilar y no se editan manualmente.

Para ejecutar la API local, exporta `SOR_MODE=development`, `SOR_ORIGIN=http://localhost:5173`, `SOR_DATABASE=./data/panel.db`, `SOR_STATIC_DIR=../frontend/dist` y una `SOR_SECRET_KEY` generada con `openssl rand -base64 32`. No reutilices esa clave en producción.

```bash
# Desde backend, con esas variables exportadas:
go run ./cmd/server create-admin --email admin@tu-dominio.com
go run ./cmd/server serve
# En otra terminal, desde la raíz:
npm run dev --prefix frontend
```

El primer comando solicita la contraseña sin mostrarla. No existe un usuario ni contraseña predeterminados. Vite publica `http://localhost:5173`; abre exactamente el origen configurado. La API corre en `127.0.0.1:8080`. Sin agente/Prometheus puedes entrar al panel y ver sus estados de conexión, pero no ejecutar operaciones del VPS.

## Producción

Sigue [la instalación](docs/installation.md). La configuración del servidor, dominios, rutas, imágenes por digest y secretos es propia del VPS; no hay valores inventados ni credenciales en el repositorio.

- [Nginx y Certbot dentro de Docker: configuración, pruebas y despliegue manual](docs/docker-edge.md)
- [Sitios externos y eliminación segura](docs/site-lifecycle.md)
- [Operación y recuperación](docs/operations.md)
- [Pruebas y validación](docs/testing.md)
- [Decisiones de implementación](docs/implementation-notes.md)
- [Contrato HTTP](docs/api/openapi.yaml)
- [Diseño técnico aprobado](docs/superpowers/specs/2026-10-27-sor-hosting-design.md)
- [Sistema visual](design-system/sor-hosting/MASTER.md)

## Comprobaciones rápidas

```bash
npm test --prefix frontend
npm run typecheck --prefix frontend
npm run build --prefix frontend
python3 tools/check-schema.py
./deploy/install-agent.sh --dry-run
```

Pruebas en navegador: `cd frontend && npx playwright install chromium && npm run test:e2e`. Usan el build de producción con respuestas controladas de la API; la prueba `npm run test:live --prefix frontend` usa los binarios Go y SQLite reales después de `make build`. `bash tools/test-docker.sh` prueba WordPress, backups y restauración en proyectos Docker desechables sin tocar Nginx/Certbot del host. Consulta [las pruebas](docs/testing.md) para los detalles.

## Alcance inicial

Una VPS; usuarios administradores; WordPress/MariaDB con plantilla conocida; backups manuales locales. No incluye correo, facturación, DNS mediante API, terminal libre ni gestión de Azure. Los backups locales necesitan una copia fuera del VPS para proteger ante pérdida de la máquina.
