terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
}

module "app" {
  source = "git::https://github.com/acme/app.git?ref=main"
}
