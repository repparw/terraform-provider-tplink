package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// Firmware is the router's model and firmware identification.
type Firmware struct {
	Model           string `json:"model"`
	HardwareVersion string `json:"hardware_version"`
	FirmwareVersion string `json:"firmware_version"`
}

// Device is one connected client as reported by the router.
type Device struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ipaddr"`
	MAC      string `json:"macaddr"`
	Type     string `json:"wire_type"`
	Active   bool   `json:"active"`
}

// Status is a practical subset of admin/status?form=all.
type Status struct {
	Model           string
	HardwareVersion string
	FirmwareVersion string
	ConnType        string
	WANIP           string
	WANGateway      string
	WANMAC          string
	LANIP           string
	LANMAC          string
	LANDHCPEnabled  bool
	CPUUsage        float64
	MemUsage        float64
	WANUptime       int
	DevicesTotal    int
	WiredTotal      int
	WirelessTotal   int
	GuestTotal      int
	IoTTotal        int
}

// DHCPReservation is one static address assignment.
type DHCPReservation struct {
	Hostname string
	MAC      string
	IP       string
	Enabled  bool
}

// PortForward is one virtual-server (port forwarding) rule.
type PortForward struct {
	Name         string
	IP           string
	ExternalPort string
	InternalPort string
	Protocol     string
	Enabled      bool
}

func asObject(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("tplink: expected an object: %w (%.200s)", err, raw)
	}
	return m, nil
}

func asList(raw json.RawMessage, envelopeKeys ...string) ([]map[string]any, error) {
	// Bare list (AX23 v2 fw 1.1.2), wrapped list ({list: [...]}, {data: [...]}),
	// or {} for an empty table.
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	obj, err := asObject(raw)
	if err != nil {
		return nil, fmt.Errorf("tplink: expected a list or an object: %.200s", raw)
	}
	if len(obj) == 0 {
		return nil, nil
	}
	for _, key := range envelopeKeys {
		if v, ok := obj[key]; ok {
			if err := json.Unmarshal(mustMarshal(v), &list); err == nil {
				return list, nil
			}
		}
	}
	return nil, fmt.Errorf("tplink: object has no list member (%v)", keysOf(obj))
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func str(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strconv.Itoa(int(t))
		}
	}
	return ""
}

func boolean(m map[string]any, key string) bool {
	switch strings.ToLower(str(m, key)) {
	case "on", "true", "yes", "1":
		return true
	}
	return false
}

// GetFirmware reads model/hardware/firmware identification.
func (c *Client) GetFirmware() (*Firmware, error) {
	raw, err := c.do("admin/firmware?form=upgrade", url.Values{"operation": {"read"}})
	if err != nil {
		return nil, err
	}
	obj, err := asObject(raw)
	if err != nil {
		return nil, err
	}
	return &Firmware{
		Model:           str(obj, "model"),
		HardwareVersion: str(obj, "hardware_version"),
		FirmwareVersion: str(obj, "firmware_version"),
	}, nil
}

// GetStatus reads a summary of the router state and connected devices.
func (c *Client) GetStatus() (*Status, error) {
	raw, err := c.do("admin/status?form=all", url.Values{"operation": {"read"}})
	if err != nil {
		return nil, err
	}
	obj, err := asObject(raw)
	if err != nil {
		return nil, err
	}
	num := func(key string) float64 {
		if v, ok := obj[key]; ok {
			if f, ok := v.(float64); ok {
				return f
			}
		}
		return 0
	}
	count := func(key string) int {
		if v, ok := obj[key].([]any); ok {
			return len(v)
		}
		return 0
	}
	st := &Status{
		Model:           str(obj, "model_name"),
		HardwareVersion: str(obj, "hardware_version"),
		FirmwareVersion: str(obj, "firmware_version"),
		ConnType:        str(obj, "wan_ipv4_conntype"),
		WANIP:           str(obj, "wan_ipv4_ipaddr"),
		WANGateway:      str(obj, "wan_ipv4_gateway"),
		WANMAC:          str(obj, "wan_macaddr"),
		LANIP:           str(obj, "lan_ipv4_ipaddr"),
		LANMAC:          str(obj, "lan_macaddr"),
		LANDHCPEnabled:  boolean(obj, "lan_ipv4_dhcp_enable"),
		CPUUsage:        num("cpu_usage"),
		MemUsage:        num("mem_usage"),
		DevicesTotal:    count("access_devices_wired") + count("access_devices_wireless_host") + count("access_devices_wireless_guest") + count("access_devices_wireless_iot"),
		WiredTotal:      count("access_devices_wired"),
		WirelessTotal:   count("access_devices_wireless_host"),
		GuestTotal:      count("access_devices_wireless_guest"),
		IoTTotal:        count("access_devices_wireless_iot"),
	}
	if f, err := strconv.Atoi(str(obj, "wan_ipv4_uptime")); err == nil {
		st.WANUptime = f
	}
	return st, nil
}

// GetDevices lists connected clients across wired, host, guest, and IoT
// wireless segments.
func (c *Client) GetDevices() ([]Device, error) {
	raw, err := c.do("admin/status?form=all", url.Values{"operation": {"read"}})
	if err != nil {
		return nil, err
	}
	obj, err := asObject(raw)
	if err != nil {
		return nil, err
	}
	var devices []Device
	for segment, typ := range map[string]string{
		"access_devices_wired":          "wired",
		"access_devices_wireless_host":  "wifi",
		"access_devices_wireless_guest": "guest",
		"access_devices_wireless_iot":   "iot",
	} {
		list, ok := obj[segment].([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			wireType := str(m, "wire_type")
			if wireType == "" {
				wireType = typ
			}
			devices = append(devices, Device{
				Hostname: str(m, "hostname"),
				IP:       str(m, "ipaddr"),
				MAC:      str(m, "macaddr"),
				Type:     wireType,
				Active:   true,
			})
		}
	}
	return devices, nil
}

// GetDHCPReservations lists the static DHCP address assignments.
func (c *Client) GetDHCPReservations() ([]DHCPReservation, error) {
	raw, err := c.do("admin/dhcps?form=reservation", url.Values{"operation": {"load"}})
	if err != nil {
		return nil, err
	}
	list, err := asList(raw, "data", "list")
	if err != nil {
		return nil, err
	}
	out := make([]DHCPReservation, 0, len(list))
	for _, m := range list {
		out = append(out, DHCPReservation{
			Hostname: str(m, "hostname"),
			MAC:      str(m, "mac"),
			IP:       str(m, "ip"),
			Enabled:  boolean(m, "enable"),
		})
	}
	return out, nil
}

// AddDHCPReservation creates a static address assignment.
func (c *Client) AddDHCPReservation(r DHCPReservation) error {
	row := map[string]any{
		"mac":      NormalizeMAC(r.MAC),
		"ip":       r.IP,
		"hostname": r.Hostname,
		"comment":  "",
		"enable":   onOff(r.Enabled),
	}
	newJSON, _ := json.Marshal(row)
	return c.writeTable("admin/dhcps?form=reservation",
		url.Values{"operation": {"insert"}, "new": {string(newJSON)}})
}

// DeleteDHCPReservation removes the reservation matching a MAC address.
func (c *Client) DeleteDHCPReservation(mac string) error {
	key, index, err := c.findReservation(NormalizeMAC(mac))
	if err != nil {
		return err
	}
	return c.writeTable("admin/dhcps?form=reservation", url.Values{
		"operation": {"remove"}, "key": {key}, "index": {strconv.Itoa(index)},
	})
}

// UpdateDHCPReservation changes an existing reservation identified by its
// current MAC. Set MAC to the new address; the router matches rows by the
// old payload.
func (c *Client) UpdateDHCPReservation(currentMAC string, r DHCPReservation) error {
	key, index, old, err := c.findReservationFull(NormalizeMAC(currentMAC))
	if err != nil {
		return err
	}
	newRow := map[string]any{
		"mac":      NormalizeMAC(r.MAC),
		"ip":       r.IP,
		"hostname": r.Hostname,
		"comment":  str(old, "comment"),
		"enable":   onOff(r.Enabled),
	}
	newJSON, _ := json.Marshal(newRow)
	oldJSON, _ := json.Marshal(old)
	return c.writeTable("admin/dhcps?form=reservation", url.Values{
		"operation": {"update"},
		"key":       {key},
		"index":     {strconv.Itoa(index)},
		"new":       {string(newJSON)},
		"old":       {string(oldJSON)},
	})
}

func (c *Client) findReservation(mac string) (key string, index int, err error) {
	key, index, _, err = c.findReservationFull(mac)
	return key, index, err
}

func (c *Client) findReservationFull(mac string) (key string, index int, row map[string]any, err error) {
	raw, err := c.do("admin/dhcps?form=reservation", url.Values{"operation": {"load"}})
	if err != nil {
		return "", 0, nil, err
	}
	list, err := asList(raw, "data", "list")
	if err != nil {
		return "", 0, nil, err
	}
	for i, m := range list {
		if NormalizeMAC(str(m, "mac")) == mac {
			return str(m, "mac"), i, m, nil
		}
	}
	return "", 0, nil, &RouterError{Path: "admin/dhcps?form=reservation",
		Code: "not found: reservation " + mac}
}

// GetPortForwards lists the virtual-server (port forwarding) rules.
func (c *Client) GetPortForwards() ([]PortForward, error) {
	raw, err := c.do("admin/nat?form=vs", url.Values{"operation": {"load"}})
	if err != nil {
		return nil, err
	}
	list, err := asList(raw, "data", "list")
	if err != nil {
		return nil, err
	}
	out := make([]PortForward, 0, len(list))
	for _, m := range list {
		out = append(out, PortForward{
			Name:         str(m, "name"),
			IP:           str(m, "ipaddr"),
			ExternalPort: str(m, "external_port"),
			InternalPort: str(m, "internal_port"),
			Protocol:     str(m, "protocol"),
			Enabled:      boolean(m, "enable"),
		})
	}
	return out, nil
}

func validatePort(p string, allowRange bool) error {
	parts := []string{p}
	if allowRange {
		parts = strings.Split(p, "-")
		if len(parts) > 2 {
			return fmt.Errorf("invalid port %q", p)
		}
	} else if strings.Contains(p, "-") {
		return fmt.Errorf("internal port must be a single port, got %q", p)
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid port %q", p)
		}
	}
	return nil
}

func normalizeProtocol(p string) (string, error) {
	switch strings.ToUpper(p) {
	case "TCP", "UDP", "ALL":
		return strings.ToUpper(p), nil
	}
	return "", fmt.Errorf("protocol must be TCP, UDP or ALL, got %q", p)
}

// IsMAC reports whether s is a parseable MAC address in colon, dash, or
// Cisco dotted format (exactly 12 hex digits with only separators between).
func IsMAC(s string) bool {
	hexDigits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			hexDigits++
		case r == ':' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return hexDigits == 12
}

// NormalizeMAC converts common MAC formats (colon, dash, or Cisco dotted)
// to the dash-uppercase form the router stores. Unparsable input is
// returned uppercased unchanged.
func NormalizeMAC(mac string) string {
	var hexDigits strings.Builder
	for _, r := range mac {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			hexDigits.WriteRune(unicode.ToUpper(r))
		case r == ':' || r == '-' || r == '.':
			// separator
		default:
			return strings.ToUpper(mac)
		}
	}
	hexed := hexDigits.String()
	if len(hexed) != 12 {
		return strings.ToUpper(mac)
	}
	var out strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			out.WriteByte('-')
		}
		out.WriteString(hexed[i : i+2])
	}
	return out.String()
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// AddPortForward creates a virtual-server rule.
func (c *Client) AddPortForward(r PortForward) error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("port forward name must not be empty")
	}
	protocol, err := normalizeProtocol(r.Protocol)
	if err != nil {
		return err
	}
	if err := validatePort(r.ExternalPort, true); err != nil {
		return err
	}
	if err := validatePort(r.InternalPort, false); err != nil {
		return err
	}
	row := map[string]any{
		"name":          r.Name,
		"ipaddr":        r.IP,
		"external_port": r.ExternalPort,
		"internal_port": r.InternalPort,
		"protocol":      protocol,
		"enable":        onOff(r.Enabled),
	}
	newJSON, _ := json.Marshal(row)
	return c.writeTable("admin/nat?form=vs",
		url.Values{"operation": {"insert"}, "new": {string(newJSON)}})
}

// UpdatePortForward changes an existing rule identified by its current name.
// The router matches the row by the old payload (the web UI regenerates row
// keys client-side on every render).
func (c *Client) UpdatePortForward(currentName string, r PortForward) error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("port forward name must not be empty")
	}
	protocol, err := normalizeProtocol(r.Protocol)
	if err != nil {
		return err
	}
	if err := validatePort(r.ExternalPort, true); err != nil {
		return err
	}
	if err := validatePort(r.InternalPort, false); err != nil {
		return err
	}
	raw, err := c.do("admin/nat?form=vs", url.Values{"operation": {"load"}})
	if err != nil {
		return err
	}
	list, err := asList(raw, "data", "list")
	if err != nil {
		return err
	}
	var old map[string]any
	var index int
	for i, m := range list {
		if str(m, "name") == currentName {
			old, index = m, i
			break
		}
	}
	if old == nil {
		return &RouterError{Path: "admin/nat?form=vs", Code: "not found: port forward " + currentName}
	}
	newRow := map[string]any{
		"name":          r.Name,
		"ipaddr":        r.IP,
		"external_port": r.ExternalPort,
		"internal_port": r.InternalPort,
		"protocol":      protocol,
		"enable":        onOff(r.Enabled),
	}
	newJSON, _ := json.Marshal(newRow)
	oldJSON, _ := json.Marshal(old)
	return c.writeTable("admin/nat?form=vs", url.Values{
		"operation": {"update"},
		"key":       {currentName},
		"index":     {strconv.Itoa(index)},
		"new":       {string(newJSON)},
		"old":       {string(oldJSON)},
	})
}

// DeletePortForward removes the rule with the given name.
func (c *Client) DeletePortForward(name string) error {
	raw, err := c.do("admin/nat?form=vs", url.Values{"operation": {"load"}})
	if err != nil {
		return err
	}
	list, err := asList(raw, "data", "list")
	if err != nil {
		return err
	}
	for i, m := range list {
		if str(m, "name") == name {
			return c.writeTable("admin/nat?form=vs", url.Values{
				"operation": {"remove"}, "key": {name}, "index": {strconv.Itoa(i)},
			})
		}
	}
	return &RouterError{Path: "admin/nat?form=vs", Code: "not found: port forward " + name}
}

func (c *Client) writeTable(path string, form url.Values) error {
	_, err := c.do(path, form)
	return err
}
