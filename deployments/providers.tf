terraform {
  required_providers {
    linode = {
      source = "linode/linode"
      version = "1.27.1"
    }

    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "linode" {
  token = local.my_token
}

provider "aws" {
  region = "us-east-1"
}
