# Dev environment: 2-node k3s (1 on-demand server + 1 spot agent), ECR, GitHub
# OIDC deploy role, budget alarm. Kafka (Redpanda) and Postgres run IN-CLUSTER
# (Helm, Phase 8) — no MSK / RDS / dedicated Kafka EC2 / ALB / NAT by design.
# Estimated ~$19-20/mo on-demand, ~$0 on credits.

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

  default_tags {
    tags = local.tags
  }
}

locals {
  name = "ecommerce-dev"
  tags = {
    Project     = "ecommerce"
    Environment = "dev"
    ManagedBy   = "terraform"
  }
  services = ["bff", "product", "order", "inventory", "payment", "notification"]
}

module "network" {
  source = "../../modules/network"

  name       = local.name
  region     = var.region
  admin_cidr = var.admin_cidr
  tags       = local.tags
}

module "compute" {
  source = "../../modules/compute"

  name                   = local.name
  region                 = var.region
  subnet_ids             = module.network.public_subnet_ids
  node_security_group_id = module.network.node_security_group_id
  server_instance_type   = var.server_instance_type
  agent_instance_type    = var.agent_instance_type
  agent_count            = var.agent_count
  tags                   = local.tags
}

module "ecr" {
  source = "../../modules/ecr"

  name     = local.name
  services = local.services
  tags     = local.tags
}

module "iam" {
  source = "../../modules/iam"

  name                = local.name
  region              = var.region
  github_repository   = var.github_repository
  ecr_repository_arns = module.ecr.repository_arns
  tags                = local.tags
}

module "budgets" {
  source = "../../modules/budgets"

  name        = local.name
  limit_usd   = var.budget_limit_usd
  alert_email = var.alert_email
}
