#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
: "${SOR_IMAGE:?Indica la imagen GHCR por digest}"
[[ "$SOR_IMAGE" =~ ^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$ ]] || { echo 'Imagen inválida: se requiere digest GHCR.' >&2; exit 1; }
[[ -f .env ]] || { echo 'Falta deploy/.env del servidor.' >&2; exit 1; }
compose=(docker compose --env-file .env -f compose.yaml)
"${compose[@]}" config --quiet
current=$("${compose[@]}" ps -q panel)
old=''
if [[ -n "$current" ]]; then
  old=$(docker inspect --format '{{.Config.Image}}' "$current")
  "${compose[@]}" exec -T panel /sor-server backup-db --destination "/data/predeploy-$(date -u +%Y%m%dT%H%M%SZ).db"
fi
"${compose[@]}" pull panel
"${compose[@]}" up -d --no-deps panel
healthy=false
for _ in $(seq 1 40); do
  current=$("${compose[@]}" ps -q panel)
  state=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$current")
  if [[ "$state" == healthy ]]; then healthy=true; break; fi
  sleep 3
done
if [[ "$healthy" == true ]]; then printf '%s\n' 'SOR Hosting desplegado y ready.'; exit 0; fi
printf '%s\n' 'Falló readiness; comprobando compatibilidad del rollback.' >&2
if [[ -n "$old" ]]; then
  max_schema=$(docker image inspect --format '{{index .Config.Labels "io.sor.schema-max"}}' "$old")
  actual_schema=$("${compose[@]}" run --rm --no-deps panel schema-version)
  if [[ "$max_schema" =~ ^[0-9]+$ && "$actual_schema" =~ ^[0-9]+$ ]] && (( actual_schema <= max_schema )); then
    SOR_IMAGE="$old" "${compose[@]}" up -d --no-deps panel
    printf '%s\n' 'Imagen anterior restaurada. Revisar el despliegue fallido.' >&2
  else
    printf '%s\n' 'Esquema incompatible: recuperación manual con el snapshot predeploy; no se degrada la base automáticamente.' >&2
  fi
fi
exit 1
