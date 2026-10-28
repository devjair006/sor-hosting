#!/usr/bin/env sh
set -eu
nginx -t
nginx -s reload
