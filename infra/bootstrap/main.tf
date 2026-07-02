# Remote-state bootstrap: S3 bucket + DynamoDB lock table. Applied ONCE with
# local state before anything else; envs/dev then uses the S3 backend.
#   terraform -chdir=infra/bootstrap apply -var state_bucket=<globally-unique>
terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  description = "AWS region for the state resources"
  type        = string
  default     = "us-east-1"
}

variable "state_bucket" {
  description = "Globally-unique name for the Terraform state bucket"
  type        = string
}

resource "aws_s3_bucket" "state" {
  bucket = var.state_bucket

  # State must survive `make destroy`; delete manually when retiring the project.
  lifecycle {
    prevent_destroy = true
  }

  tags = {
    Project = "ecommerce"
    Purpose = "terraform-state"
  }
}

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket                  = aws_s3_bucket.state.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_dynamodb_table" "lock" {
  name         = "ecommerce-terraform-lock"
  billing_mode = "PAY_PER_REQUEST" # pennies at this usage
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  tags = {
    Project = "ecommerce"
    Purpose = "terraform-state-lock"
  }
}

output "state_bucket" {
  value = aws_s3_bucket.state.bucket
}

output "lock_table" {
  value = aws_dynamodb_table.lock.name
}
