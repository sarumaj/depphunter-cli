log_info() {
  printf '%s\n' "$*"
}
log_warn() ( echo "warn: $*" >&2 )
