locals {
  org = "acme"
}

remote_state {
  backend = "s3"
  config = {
    bucket = "${local.org}-state"
  }
}
