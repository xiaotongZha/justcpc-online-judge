#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
release_dir="$(cd -- "$script_dir/.." && pwd)"
run_user="${SUDO_USER:-${USER:-}}"
start_service=1
print_only=0
usage() {
    printf 'Usage: sudo ./install-service.sh [--run-user USER] [--no-start]\n       ./install-service.sh --print [--run-user USER]\n'
}
while (($#)); do
    case "$1" in
        --run-user)
            [[ $# -ge 2 && -n "$2" ]] || { usage >&2; exit 2; }
            run_user="$2"; shift 2 ;;
        --no-start) start_service=0; shift ;;
        --print) print_only=1; shift ;;
        --help|-h) usage; exit 0 ;;
        *) usage >&2; exit 2 ;;
    esac
done
if [[ -z "$run_user" || "$run_user" == root ]] || ! id "$run_user" >/dev/null 2>&1; then
    printf 'Use an existing ordinary account: --run-user USER\n' >&2; exit 1
fi
if [[ "$release_dir" =~ [[:space:]%] ]]; then
    printf 'The deployment path must not contain whitespace or %% characters.\n' >&2; exit 1
fi
binary="$release_dir/bin/justcpc-online-judge"
if [[ ! -x "$binary" || ! -f "$release_dir/web/dist/index.html" ]]; then
    printf 'Release is incomplete: binary or web/dist/index.html is missing.\n' >&2; exit 1
fi
unit() {
    cat <<EOF
[Unit]
Description=JustCPC Online Judge
After=network.target justcpc-judge-engine.service

[Service]
Type=simple
User=$run_user
Group=$(id -gn "$run_user")
WorkingDirectory=$release_dir
ExecStart=$binary
Environment=GIN_MODE=release
Restart=on-failure
RestartSec=2
KillMode=control-group

[Install]
WantedBy=multi-user.target
EOF
}
if ((print_only)); then unit; exit 0; fi
if ((EUID != 0)); then printf 'Run with sudo.\n' >&2; exit 1; fi
if [[ ! -d /run/systemd/system ]] || ! command -v systemctl >/dev/null 2>&1; then
    printf 'A systemd host is required.\n' >&2; exit 1
fi
temporary="$(mktemp)"
trap 'rm -f -- "$temporary"' EXIT
unit > "$temporary"
install -o root -g root -m 0644 "$temporary" /etc/systemd/system/justcpc-online-judge.service
systemctl daemon-reload
systemctl enable justcpc-online-judge.service >/dev/null
if ((start_service)); then
    systemctl restart justcpc-online-judge.service
    systemctl is-active --quiet justcpc-online-judge.service || {
        printf 'Inspect: journalctl -u justcpc-online-judge.service\n' >&2; exit 1;
    }
fi
printf 'Registered justcpc-online-judge.service for %s.\n' "$run_user"
