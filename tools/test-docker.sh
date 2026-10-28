#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/go/bin:$PATH"

# Resolve official image tags once, then test only their immutable digests.
# Resources belong to random test projects; cleanup never prunes the host.
images=("${SOR_TEST_WP_IMAGE:-wordpress:php8.3-apache}" "${SOR_TEST_DB_IMAGE:-mariadb:11.4}" "${SOR_TEST_CLI_IMAGE:-wordpress:cli-php8.3}")
digests=()
for image in "${images[@]}"; do
  docker pull "$image"
  digest=$(docker image inspect "$image" --format '{{index .RepoDigests 0}}')
  [[ "$digest" =~ @sha256:[a-f0-9]{64}$ ]]
  digests+=("$digest")
done
export SOR_DOCKER_INTEGRATION=1
export SOR_TEST_WP_IMAGE="${digests[0]}"
export SOR_TEST_DB_IMAGE="${digests[1]}"
export SOR_TEST_CLI_IMAGE="${digests[2]}"
.tools/bin/sqlc generate -f agent/sqlc.yaml
cd agent
go test -tags=integration ./internal/backups -run TestDockerWordPressBackupRestore -count=1 -v -timeout=15m
