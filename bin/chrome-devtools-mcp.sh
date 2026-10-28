#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

node_is_supported() {
  "$1" -e 'const [a,b]=process.versions.node.split(".").map(Number);process.exit(a>=23||(a===22&&b>=12)||(a===20&&b>=19)?0:1)' 2>/dev/null
}

node_bin="$(command -v node || true)"
if [[ -z "$node_bin" ]] || ! node_is_supported "$node_bin"; then
  export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
  if [[ -s "$NVM_DIR/nvm.sh" ]]; then
    # shellcheck disable=SC1091
    . "$NVM_DIR/nvm.sh" >/dev/null
    nvm use --silent "$(cat "$root/.nvmrc")" >/dev/null || true
    node_bin="$(command -v node || true)"
  fi
fi

if [[ -z "$node_bin" ]] || ! node_is_supported "$node_bin"; then
  echo "pilotab: chrome-devtools-mcp needs Node ^20.19 || ^22.12 || >=23 (found: $(${node_bin:-node} -v 2>/dev/null || echo none))" >&2
  exit 1
fi

exec "$node_bin" "$root/node_modules/chrome-devtools-mcp/build/src/bin/chrome-devtools-mcp.js" --no-usage-statistics --no-performance-crux "$@"
