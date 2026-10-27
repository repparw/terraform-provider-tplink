# Read-only inventory of the router. Password comes from the environment:
#   export TF_VAR_router_password="..."
variable "router_password" {
  type      = string
  sensitive = true
}

provider "tplink" {
  host     = "http://192.168.0.1"
  password = var.router_password
}

data "tplink_firmware" "current" {}

data "tplink_router" "current" {}

data "tplink_devices" "inventory" {}

data "tplink_dhcp_reservations" "all" {}

data "tplink_port_forwards" "all" {}

output "firmware" {
  value = {
    model     = data.tplink_firmware.current.model
    hardware  = data.tplink_firmware.current.hardware_version
    firmware  = data.tplink_firmware.current.firmware_version
  }
}

output "router" {
  value = {
    wan_ip     = data.tplink_router.current.wan_ip
    lan_ip     = data.tplink_router.current.lan_ip
    uptime     = data.tplink_router.current.wan_uptime
    devices    = data.tplink_router.current.devices_total
  }
}

output "reservations" {
  value = data.tplink_dhcp_reservations.all.reservations
}

output "port_forwards" {
  value = data.tplink_port_forwards.all.rules
}
