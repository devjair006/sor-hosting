#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/go/bin:$PATH"
images=("${SOR_TEST_NGINX_IMAGE:-nginx:stable-alpine}" "${SOR_TEST_CERTBOT_IMAGE:-certbot/certbot:v5.8.0}")
digests=()
for image in "${images[@]}"; do
 docker pull "$image"
 digest=$(docker image inspect "$image" --format '{{index .RepoDigests 0}}')
 [[ "$digest" =~ @sha256:[a-f0-9]{64}$ ]]
 digests+=("$digest")
done
export SOR_EDGE_INTEGRATION=1
export SOR_TEST_NGINX_IMAGE="${digests[0]}"
export SOR_TEST_CERTBOT_IMAGE="${digests[1]}"
(cd agent && go test -tags=integration ./internal/dockerexec -count=1 -v)
bash tools/test-docker.sh
