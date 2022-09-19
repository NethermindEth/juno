variable "token" {}
variable "authorized_keys" {}
variable "root_pass" {}
variable "region" {
  default = "us-southeast"
}

variable "nodes_script_file" {
  type        = string
  description = "Script file path"
  default     = "./run-juno.sh"
}