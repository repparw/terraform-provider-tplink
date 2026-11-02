terraform {
  required_providers {
    tplink = {
      source = "registry.opentofu.org/repparw/tplink"
    }
  }
}

variable "router_password" {
  type      = string
  sensitive = true
}

provider "tplink" {
  host     = "http://192.168.0.1"
  password = var.router_password
}
