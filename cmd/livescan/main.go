// Command livescan is a read-only sanity check of the Go client against a
// real router. It performs one login, a set of read-only queries, and a
// logout. Sensitive values (passphrases, WAN addresses) are redacted.
//
//	go run ./cmd/livescan
//
// Environment: TPLINK_HOST (default http://192.168.0.1), TPLINK_PASSWORD.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

func main() {
	password := os.Getenv("TPLINK_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "TPLINK_PASSWORD not set")
		os.Exit(2)
	}
	host := os.Getenv("TPLINK_HOST")
	if host == "" {
		host = "http://192.168.0.1"
	}
	cl, err := client.New(client.Config{Host: host, Password: password, TLSSkipVerify: true})
	if err != nil {
		fail(err)
	}
	report := map[string]string{}

	fw, err := cl.GetFirmware()
	if err == nil {
		report["firmware"] = fmt.Sprintf("%s / %s / %s", fw.Model, fw.HardwareVersion, fw.FirmwareVersion)
	} else {
		report["firmware"] = "ERROR: " + err.Error()
	}

	st, err := cl.GetStatus()
	if err == nil {
		report["status"] = fmt.Sprintf("wan=%s (uptime %ds) lan=%s cpu=%.2f mem=%.2f clients=%d",
			redactIP(st.WANIP), st.WANUptime, st.LANIP, st.CPUUsage, st.MemUsage, st.DevicesTotal)
	} else {
		report["status"] = "ERROR: " + err.Error()
	}

	devices, err := cl.GetDevices()
	if err == nil {
		report["devices"] = fmt.Sprintf("%d devices", len(devices))
	} else {
		report["devices"] = "ERROR: " + err.Error()
	}

	res, err := cl.GetDHCPReservations()
	if err == nil {
		names := make([]string, 0, len(res))
		for _, r := range res {
			names = append(names, r.Hostname+"="+r.IP)
		}
		report["dhcp_reservations"] = fmt.Sprintf("%d: %s", len(res), strings.Join(names, ", "))
	} else {
		report["dhcp_reservations"] = "ERROR: " + err.Error()
	}

	fwds, err := cl.GetPortForwards()
	if err == nil {
		names := make([]string, 0, len(fwds))
		for _, f := range fwds {
			names = append(names, fmt.Sprintf("%s:%s->%s:%s(%s)", f.Name, f.ExternalPort, f.IP, f.InternalPort, f.Protocol))
		}
		report["port_forwards"] = fmt.Sprintf("%d: %s", len(fwds), strings.Join(names, ", "))
	} else {
		report["port_forwards"] = "ERROR: " + err.Error()
	}

	_ = cl.Logout()
	for _, key := range []string{"firmware", "status", "devices", "dhcp_reservations", "port_forwards"} {
		fmt.Printf("%-18s %s\n", key, report[key])
	}
	if strings.Contains(report["firmware"]+report["status"], "ERROR") {
		os.Exit(1)
	}
}

func redactIP(s string) string {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) == 3 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}
