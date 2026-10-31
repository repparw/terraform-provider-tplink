resource "tplink_port_forward" "web" {
  name          = "web"
  ip            = "192.168.0.4"
  external_port = "443"
  internal_port = "443"
  protocol      = "TCP"
  enabled       = true
}
