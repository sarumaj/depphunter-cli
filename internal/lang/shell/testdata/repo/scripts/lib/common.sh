# Shared helpers, sourced only.
COMMON_LOADED=1
die() { echo "$*" >&2; exit 1; }
alias ll='ls -l'
source "$(dirname "${BASH_SOURCE[0]}")/log.bash"
