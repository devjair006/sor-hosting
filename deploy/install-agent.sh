#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "${1:-}" == --dry-run ]]; then
  printf '%s\n' 'Creará directorios SOR, instalará bin/sor-agent, configuración de ejemplo y unidad systemd.' 'No modificará nginx.conf, no emitirá certificados ni reiniciará servicios existentes.'
  exit 0
fi
[[ "${EUID}" == 0 ]] || { echo 'Ejecuta como root en el VPS de destino.' >&2; exit 1; }
[[ -x bin/sor-agent ]] || { echo 'Falta bin/sor-agent. Ejecuta make build primero.' >&2; exit 1; }
getent group sor-panel >/dev/null || groupadd --gid 10001 sor-panel
[[ "$(getent group sor-panel | cut -d: -f3)" == 10001 ]] || { echo 'GID sor-panel incompatible.' >&2; exit 1; }
install -d -m 0700 /etc/sor-hosting /var/lib/sor-hosting/backups /var/lib/sor-hosting/agent /var/lib/sor-hosting/sites
install -d -m 0711 /var/lib/sor-hosting
install -d -m 0755 /etc/nginx/sor-hosting.d /var/lib/sor-hosting/acme
install -d -o 10001 -g 10001 -m 0700 /var/lib/sor-hosting/panel
install -d -o root -g 10001 -m 0750 /run/sor-hosting
install -m 0755 bin/sor-agent /usr/local/sbin/sor-hosting-agent
if [[ ! -f /etc/sor-hosting/agent.key ]]; then umask 077; openssl rand -base64 32 > /etc/sor-hosting/agent.key; fi
if [[ ! -f /etc/sor-hosting/agent.json ]]; then install -m 0600 deploy/agent.example.json /etc/sor-hosting/agent.json; fi
install -m 0644 deploy/systemd/sor-hosting-agent.service /etc/systemd/system/sor-hosting-agent.service
systemctl daemon-reload
printf '%s\n' 'Instalado. Revisa agent.json, configura imágenes por digest e incluye sor-hosting.d en Nginx.' 'Después: nginx -t && systemctl enable --now sor-hosting-agent'
