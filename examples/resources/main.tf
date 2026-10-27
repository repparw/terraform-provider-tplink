# Managed DHCP reservations and port-forwarding rules.
# Password comes from the environment: export TF_VAR_router_password="..."
variable "router_password" {
  type      = string
  sensitive = true
}

provider "tplink" {
  host     = "http://192.168.0.1"
  password = var.router_password
}

resource "tplink_dhcp_reservation" "printer" {
  mac      = "AA-BB-CC-DD-EE-FF"
  ip       = "192.168.0.50"
  hostname = "printer"
  enabled  = true
}

resource "tplink_port_forward" "web" {
  name          = "web"
  ip            = "192.168.0.4"
  external_port = "443"
  internal_port = "443"
  protocol      = "TCP"
  enabled       = true
}

# Rules the LAN depends on (e.g. the gateway's own forwarding) should carry
# lifecycle blocks so `tofu apply` can never destroy them in an unattended
# run. The router's single-admin session also means applies interrupt any
# interactive web-UI login — review plans before applying.
# lifecycle {
#   prevent_destroy = true
# }
