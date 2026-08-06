terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.31.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.5.1"
}

module "internal" {
  source = "git::https://github.com/acme/tf-modules.git//vpc?ref=v2.3.0"
}

module "local" {
  source = "./modules/naming"
}
