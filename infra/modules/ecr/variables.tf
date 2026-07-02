variable "name" {
  description = "Registry namespace prefix"
  type        = string
}

variable "services" {
  description = "Service names — one repo each"
  type        = list(string)
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default     = {}
}
