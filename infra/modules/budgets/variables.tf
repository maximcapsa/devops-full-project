variable "name" {
  description = "Name prefix"
  type        = string
}

variable "limit_usd" {
  description = "Monthly budget ceiling in USD"
  type        = number
  default     = 20
}

variable "alert_email" {
  description = "Email that receives budget alerts"
  type        = string
}
