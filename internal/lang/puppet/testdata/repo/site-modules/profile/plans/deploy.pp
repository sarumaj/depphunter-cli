plan profile::deploy(TargetSpec $targets) {
  run_task('profile::restart', $targets)
}
