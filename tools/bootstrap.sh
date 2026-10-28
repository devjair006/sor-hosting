#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/go/bin:$PATH"
command -v go >/dev/null || { echo 'Instala Go 1.26 desde https://go.dev/dl/ y vuelve a ejecutar este script.' >&2; exit 1; }
mkdir -p .tools/bin
GOWORK=off GOBIN="$PWD/.tools/bin" go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
(cd backend && ../.tools/bin/sqlc generate)
(cd agent && ../.tools/bin/sqlc generate)
for module in shared backend agent; do (cd "$module" && GOWORK=off go mod tidy); done
npm ci --prefix frontend
printf 'Toolchain y dependencias preparados. Ejecuta make check y make build.\n'
