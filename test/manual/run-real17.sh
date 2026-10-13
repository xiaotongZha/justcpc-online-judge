#!/usr/bin/env bash
set -euo pipefail

# This script owns every temporary path it creates. The trap also runs after a
# failed import or a failed judge request, so no fixture directory or ZIP is
# left for an operator to clean manually.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
oj_root="$(cd "$script_dir/../.." && pwd)"
repo_root="$(cd "$oj_root/../.." && pwd)"
work_root="$oj_root/test/manual/.work"
mkdir -p "$work_root"
work_dir="$(mktemp -d "$work_root/real17.XXXXXX")"
cleanup() {
  case "$work_dir" in
    "$work_root"/real17.*) rm -rf -- "$work_dir" ;;
    *) echo "refusing to remove unexpected temporary path: $work_dir" >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

package_path="$work_dir/contest-17-real-sample.zip"
cookie_path="$work_dir/session.cookie"
server_url="${OJ_URL:-http://127.0.0.1:18081}"

: "${OJ_ADMIN_PASSWORD:?set OJ_ADMIN_PASSWORD before running this script}"

cd "$oj_root"
go run ./test/manual/build17 \
  -output "$package_path" \
  -data-root "$repo_root/比赛数据/17届比赛数据"

curl --fail-with-body --silent --show-error \
  --cookie-jar "$cookie_path" \
  --header 'Content-Type: application/json' \
  --data '{"username":"admin","password":"'"$OJ_ADMIN_PASSWORD"'"}' \
  "$server_url/api/login" >/dev/null

import_response="$(curl --fail-with-body --silent --show-error \
  --cookie "$cookie_path" \
  --form "file=@$package_path;type=application/zip" \
  "$server_url/api/contests/import")"
printf '%s\n' "$import_response"
