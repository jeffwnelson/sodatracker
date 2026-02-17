terraform {
  required_version = ">= 1.11.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket  = "sodatracker-tfstate-136532201934"
    key     = "sodatracker/terraform.tfstate"
    region  = "us-west-2"
    profile = "personal"
  }
}

provider "aws" {
  region  = "us-west-2"
  profile = "personal"

  default_tags {
    tags = {
      Project     = "SodaTracker"
      Environment = var.environment
      ManagedBy   = "OpenTofu"
    }
  }
}

# CloudFront requires ACM certs in us-east-1 (not needed yet, but alias provider is here for future use)
provider "aws" {
  alias   = "us_east_1"
  region  = "us-east-1"
  profile = "personal"
}
