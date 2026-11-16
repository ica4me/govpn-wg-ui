package console

import (
	"fmt"
	"strconv"
	"strings"
)

func current(b bool) string {
	if b {
		return "page"
	}
	return "false"
}
func symbol(k string) string {
	switch k {
	case "dashboard":
		return "▦"
	case "sites":
		return "⇄"
	case "clients":
		return "♙"
	case "groups":
		return "▤"
	default:
		return "⚙"
	}
}
func singular(k string) string {
	switch k {
	case "sites":
		return "site"
	case "clients":
		return "client"
	default:
		return "group"
	}
}
func subtitle(k string) string {
	switch k {
	case "sites":
		return "Connect branch networks to your hub. Control exactly which destinations they can reach."
	case "clients":
		return "Provision devices, assign multi-site access, and deploy a single WireGuard configuration."
	case "groups":
		return "Organize client interfaces with dedicated subnets, UDP ports, and MTU settings."
	case "system":
		return "Inspect engine configuration and deliberately run maintenance operations."
	case "account":
		return "Change the administrator username and password. Saving signs out every session."
	default:
		return "Your network at a glance. Live state from the GoVPN control plane."
	}
}
func human(k string) string {
	m := map[string]string{"endpoint": "Public endpoint", "public_interface": "Public interface", "server_lan": "Server LAN", "uptime_seconds": "Uptime GoVPN Service", "server_uptime_seconds": "Server Uptime", "last_apply": "Last applied", "last_apply_error": "Last apply error", "default_mtu": "Default MTU", "base_port": "Base UDP port", "mss_mode": "MSS mode", "tune": "Kernel & NIC tuning", "reconcile": "Reconcile state"}
	if s := m[k]; s != "" {
		return s
	}
	return strings.ReplaceAll(k, "_", " ")
}
func value(r Record, k string) string {
	if r == nil {
		return "—"
	}
	v := r.Text(k)
	if k == "uptime_seconds" || k == "server_uptime_seconds" {
		seconds, err := strconv.ParseFloat(v, 64)
		if err != nil || seconds < 0 {
			return "—"
		}
		minutes := int64(seconds / 60)
		return fmt.Sprintf("%d:%02d:%02d", minutes/1440, minutes/60%24, minutes%60)
	}
	if v == "" {
		if k == "last_apply_error" {
			return "None reported"
		}
		return "—"
	}
	return v
}
func count(r []Record) string { return fmt.Sprint(len(r)) }
func network(r Record) string {
	if r.Text("address") != "" {
		return r.Text("address")
	}
	return r.Text("subnet")
}
func targets(r Record, k string) string {
	key := "target_sites"
	if k == "sites" {
		key = "allowed_target_sites"
	}
	s := r.Text(key)
	if s == "" {
		return "No destination sites"
	}
	return s
}
func formAction(p Page) string {
	if p.Edit {
		return "/" + p.Kind + "/" + p.ID
	}
	return "/" + p.Kind + "/new"
}
func maintenanceHint(k string) string {
	if k == "tune" {
		return "Apply kernel and NIC tuning. The backend returns applied, unchanged, unsupported or failed results."
	}
	return "Re-detect the public NIC and endpoint, then reapply interfaces, peers, routes and firewall state."
}
