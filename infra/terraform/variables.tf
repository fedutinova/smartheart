variable "hcloud_token" {
  description = "Hetzner Cloud API token (generate at console.hetzner.cloud → Security → API Tokens)"
  type        = string
  sensitive   = true
}

variable "domain_name" {
  description = "Root domain for the app, e.g. smartheart.io"
  type        = string
}

variable "ssh_public_key" {
  description = "SSH public key content (contents of ~/.ssh/id_ed25519.pub)"
  type        = string
}

variable "server_type" {
  description = "Hetzner server type. CX32 (4 vCPU, 8 GB) is the recommended minimum for RAG."
  type        = string
  default     = "cpx32"
}

variable "server_location" {
  description = "Hetzner datacenter: nbg1 (Nuremberg), fsn1 (Falkenstein), hel1 (Helsinki), ash (Ashburn), hil (Hillsboro)"
  type        = string
  default     = "nbg1"
}

variable "environment" {
  description = "Deployment environment label"
  type        = string
  default     = "production"
}
