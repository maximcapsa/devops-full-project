# Remote state in S3 with DynamoDB locking. The bucket/table come from
# infra/bootstrap (apply that first, once). The bucket name is account-specific,
# so it's supplied at init time:
#   terraform init -backend-config="bucket=<your-state-bucket>"
terraform {
  backend "s3" {
    key            = "envs/dev/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "ecommerce-terraform-lock"
    encrypt        = true
  }
}
