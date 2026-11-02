resource "tplink_dhcp_reservation" "printer" {
  mac      = "AA-BB-CC-DD-EE-FF"
  ip       = "192.168.0.50"
  hostname = "printer"
  enabled  = true
}
