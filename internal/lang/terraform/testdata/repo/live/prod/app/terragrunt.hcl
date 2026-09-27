include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "envcommon" {
  path   = "${dirname(find_in_parent_folders("root.hcl"))}/_envcommon/app.hcl"
  expose = true
}

terraform {
  source = "${include.envcommon.locals.base_source_url}?ref=v0.8.0"
}

dependency "vpc" {
  config_path = "../vpc"
}

dependencies {
  paths = ["../dns"]
}

locals {
  env = read_terragrunt_config(find_in_parent_folders("env.hcl"))
}

inputs = {
  vpc_id = dependency.vpc.outputs.vpc_id
}
