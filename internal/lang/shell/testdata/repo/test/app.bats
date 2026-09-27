#!/usr/bin/env bats
load test_helper
load 'helpers/missing'

setup() {
  source "$BATS_TEST_DIRNAME/../scripts/lib/common.sh"
}

@test "build prints usage" {
  run "$BATS_TEST_DIRNAME/../scripts/build.sh" --help
  [ "$status" -eq 0 ]
}
