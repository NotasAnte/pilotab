#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="$root/bin/.build/pilotab"

go_bin="$(command -v go || true)"
for candidate in /usr/local/go/bin/go "$HOME/.local/go/bin/go" "$HOME/go/bin/go"; do
  [[ -z "$go_bin" && -x "$candidate" ]] && go_bin="$candidate"
done

if [[ -n "$go_bin" ]]; then
  (cd "$root" && "$go_bin" build -o "$binary" ./cmd/pilotab) >&2
elif [[ ! -x "$binary" ]]; then
  echo "pilotab: Go is not installed and $binary has not been built" >&2
  exit 1
fi

config="$root/config/pilot.json"
[[ -f "$root/config/pilot.local.json" ]] && config="$root/config/pilot.local.json"

args=()
if [[ "${PILOTAB_DANGEROUSLY_SKIP_CONFIRMATIONS:-}" == "1" ]]; then
  args+=(-dangerously-skip-confirmations)
fi

exec "$binary" "${args[@]}" -config "${PILOTAB_CONFIG:-$config}" "$@"
