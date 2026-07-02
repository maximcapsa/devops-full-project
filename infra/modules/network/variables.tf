variable "name" {
  description = "Name prefix for network resources"
  type        = string
}

variable "region" {
  description = "AWS region (for the S3 endpoint service name)"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "admin_cidr" {
  description = "CIDR allowed to reach the k8s API (set to your IP/32; kubeconfig certs still required)"
  type        = string
  default     = "0.0.0.0/0"
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default     = {}
}
