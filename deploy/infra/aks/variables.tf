variable "prefix" {
  description = "Name prefix for Azure resources"
  type        = string
  default     = "kc-pim"
}

variable "location" {
  description = "Azure region"
  type        = string
  default     = "eastus"
}

variable "kubernetes_version" {
  description = "AKS Kubernetes version (null = Azure default)"
  type        = string
  default     = null
}

variable "node_count" {
  description = "Default node pool size (keep 1 for free-tier labs)"
  type        = number
  default     = 1
}

variable "vm_size" {
  description = "Node VM size — B2s fits most free-trial budgets"
  type        = string
  default     = "Standard_B2s"
}

variable "tags" {
  description = "Tags applied to all resources"
  type        = map(string)
  default = {
    project = "keycloak-pim"
    tier    = "free-lab"
  }
}
