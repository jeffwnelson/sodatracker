variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "prod"
}

variable "api_key" {
  description = "API key for Pi agent authentication"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "JWT signing secret for admin auth"
  type        = string
  sensitive   = true
}

variable "admin_password" {
  description = "Initial admin user password"
  type        = string
  sensitive   = true
}
