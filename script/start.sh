#!/usr/bin/env bash
set -euo pipefail
if (($#)); then
    [[ "$1" == "--help" && $# == 1 ]] && { printf 'Usage: ./start.sh\n'; exit 0; }
    printf 'Usage: ./start.sh\n' >&2
    exit 2
fi
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir/.."
export GIN_MODE="${GIN_MODE:-release}"
binary=./bin/justcpc-online-judge
if [[ -f go.mod ]]; then binary=./build/bin/justcpc-online-judge; fi
exec "$binary"
