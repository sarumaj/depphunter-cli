terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source = "registry.opentofu.org/hashicorp/random"
    }
    acme = { source = "tf.corp.test/acme/acme", version = "1.4.0" }
  }
}

provider "aws" {
  region = var.region
}

provider "aws" {
  alias  = "west"
  region = "us-west-2"
}

module "network" {
  source = "./modules/network"
  cidr   = var.cidr
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.1.2"
}

module "iam" {
  source  = "registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-role"
  version = "~> 6.0"
}

module "label" {
  source = "cloudposse/label/null"
}

module "private" {
  source  = "app.terraform.io/acme/db/aws"
  version = "= 2.0.1"
}

module "storage" {
  source = "git::https://github.com/acme/tf-modules.git//storage?ref=v1.2.0"
}

module "dns" {
  source = "github.com/acme/tf-dns?ref=0123456789abcdef0123456789abcdef01234567"
}

module "queue" {
  source = "git@github.com:acme/tf-queue.git"
}

module "cdn" {
  source = "s3::https://s3-eu-west-1.amazonaws.com/acme-modules/cdn.zip"
}

resource "aws_instance" "web" {
  provider  = aws.west
  ami       = data.aws_ami.ubuntu.id
  subnet_id = module.network.subnet_id
  user_data = templatefile("${path.module}/templates/init.sh.tpl", { name = local.name })
  tags      = { for sub_net in var.subnets : sub_net.name => sub_net.id }
}

resource "random_id" "suffix" {
  byte_length = 4
}

resource "acme_thing" "x" {}

resource "google_storage_bucket" "b" {
  name = "b-${random_id.suffix.hex}"
}

resource "terraform_data" "noop" {}

moved {
  from = aws_instance.old
  to   = aws_instance.web
}
