variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "admin_cidr" {
  description = "CIDR allowed to reach the k8s API — set to <your-ip>/32"
  type        = string
  default     = "0.0.0.0/0"
}

variable "github_repository" {
  description = "GitHub repo (owner/name) allowed to assume the deploy role"
  type        = string
  default     = "maximcapsa/devops-full-project"
}

variable "server_instance_type" {
  description = "k3s server instance type"
  type        = string
  default     = "t4g.small"
}

variable "agent_instance_type" {
  description = "k3s spot agent instance type"
  type        = string
  default     = "t4g.small"
}

variable "agent_count" {
  description = "Number of spot agents"
  type        = number
  default     = 1
}

variable "budget_limit_usd" {
  description = "Monthly budget ceiling (alerts at 25%, 100%, forecast)"
  type        = number
  default     = 20
}

variable "alert_email" {
  description = "Email for budget alerts"
  type        = string
}
