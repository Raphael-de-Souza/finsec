###############################################################################
# ECR module input variables
###############################################################################

variable "name" {
  description = "Name of the ECR repository"
  type        = string
}

variable "tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default     = {}
}
