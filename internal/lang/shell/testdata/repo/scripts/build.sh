#!/usr/bin/env bash
# Builds the project. "source ./not-a-real-file.sh" in a comment is not read.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(git rev-parse --show-toplevel)"
readonly VERSION="1.4.0"
export BUILD_MODE=release
tmp_dir=/tmp/build

source "$SCRIPT_DIR/lib/common.sh"
. "$(dirname "$0")/lib/log.bash"
source "${BASH_SOURCE%/*}/lib/common.sh"
source ~/.bashrc
source "$HOME/.config/build.sh"
source /etc/profile
source "$UNKNOWN_DIR/x.sh"

build() {
  local out="$1"
  "$SCRIPT_DIR/deploy" --dry-run
  bash scripts/lib/log.bash
  python3 -u "$ROOT_DIR/scripts/gen.py" --out "$out"
  cat <<EOF >"$out/manifest"
source $SCRIPT_DIR/not-sourced.sh
./not-run.sh
EOF
  echo "source lib/common.sh" 'bash ./quoted.sh'
  version=$(case "$VERSION" in 1.*) echo one ;; *) echo other ;; esac)
}

function clean {
  ./scripts/deploy --clean 2>/dev/null
}

case "${1:-}" in
  build) build dist ;;
  clean | reset) clean ;;
  *) exec ./scripts/deploy "$@" ;;
esac
