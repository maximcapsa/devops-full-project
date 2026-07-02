variable "name" {
  description = "Name prefix"
  type        = string
}

variable "region" {
  description = "AWS region"
  type        = string
}

variable "github_repository" {
  description = "GitHub repo allowed to assume the deploy role (owner/name)"
  type        = string
}

variable "ecr_repository_arns" {
  description = "ECR repository ARNs the deploy role may push to"
  type        = list(string)
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default     = {}
}
