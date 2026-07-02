variable "name" {
  description = "Name prefix (also the SSM parameter namespace)"
  type        = string
}

variable "region" {
  description = "AWS region"
  type        = string
}

variable "subnet_ids" {
  description = "Public subnet ids for nodes"
  type        = list(string)
}

variable "node_security_group_id" {
  description = "Security group for all k3s nodes"
  type        = string
}

variable "server_instance_type" {
  description = "k3s server instance type (Graviton)"
  type        = string
  default     = "t4g.small"
}

variable "agent_instance_type" {
  description = "k3s agent instance type (Graviton, spot)"
  type        = string
  default     = "t4g.small"
}

variable "agent_count" {
  description = "Number of spot agents"
  type        = number
  default     = 1
}

variable "root_volume_gb" {
  description = "Root EBS volume size per node"
  type        = number
  default     = 16
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default     = {}
}
