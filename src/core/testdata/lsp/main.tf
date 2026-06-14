terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.6.0"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "4.0.0"
}

module "pinned" {
  source = "git::https://github.com/example/module.git?ref=abc1234def5678901234567890abcdef01234567"
}
