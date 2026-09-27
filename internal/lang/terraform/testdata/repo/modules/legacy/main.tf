provider "aws" {
  version = "~> 2.0"
  region  = "eu-central-1"
}

resource "aws_s3_bucket" "logs" {
  bucket = "logs"
}

module "old_style" {
  source = "network"
}
