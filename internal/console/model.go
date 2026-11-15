package console

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Record map[string]any

func (r Record) Text(k string) string {
	v := r[k]
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []string:
		return strings.Join(x, ", ")
	case []any:
		a := []string{}
		for _, v := range x {
			a = append(a, fmt.Sprint(v))
		}
		return strings.Join(a, ", ")
	default:
		return fmt.Sprint(v)
	}
}
func (r Record) Bool(k string) bool { v, _ := r[k].(bool); return v }
func (r Record) Has(k, v string) bool {
	if a, ok := r[k].([]string); ok {
		for _, x := range a {
			if x == v {
				return true
			}
		}
	}
	a, _ := r[k].([]any)
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func (r Record) Status() string {
	if a, ok := r["runtime"].(map[string]any); ok {
		return fmt.Sprint(a["status"])
	}
	return "group"
}
func (r Record) JSON() string { b, _ := json.MarshalIndent(r, "", "  "); return string(b) }
func (r Record) Label() string {
	if s := r.Text("name"); s != "" {
		return s
	}
	return r.Text("username")
}

type Field struct {
	Key, Label, Kind, Hint string
	Required               bool
}

func Fields(kind string, edit bool) []Field {
	name := Field{"name", "Name", "text", "1–64 characters; letters, numbers, spaces, _ . @ -", true}
	port := Field{"port", "UDP port", "number", "Leave blank to allocate automatically (1–65535)", false}
	mtu := Field{"mtu", "MTU", "number", "1280–9000; blank uses backend default", false}
	mode := Field{"tunnel_mode", "Tunnel mode", "mode", "Split routes only allowed networks; full routes all IPv4 internet", false}
	enabled := Field{"enabled", "Enabled", "bool", "Disabling disconnects this peer without deleting it", false}
	rotate := Field{"rotate_keys", "Rotate keys on save", "bool", "Old configuration stops working; download and deploy again", false}
	var fs []Field
	switch kind {
	case "groups":
		fs = []Field{name, port, mtu}
	case "sites":
		fs = []Field{name, {"local_subnets", "Local LAN subnets", "list", "1–32 IPv4 CIDRs, separated by commas or newlines; minimum /8", true}, {"allow_all_subnets", "Expose all local subnets", "bool", "Otherwise only the explicitly exposed CIDRs are reachable", false}, {"exposed_subnets", "Exposed subnets", "list", "Must be contained within local LAN subnets", false}, {"can_access_server_lan", "Allow access to server LAN", "bool", "One-way access from this site to the server LAN", false}, {"allowed_target_sites", "Allowed destination sites", "sites", "Select any number of sites. Access is one-way; self-selection is excluded", false}, mode, port, mtu, enabled}
	case "clients":
		if !edit {
			fs = append(fs, Field{"username", "Username", "text", "Immutable after creation; letters, numbers, _ . @ -", true}, Field{"group", "Client group", "group", "Immutable after creation; default is created automatically if needed", false})
		}
		fs = append(fs, Field{"target_sites", "Allowed destination sites", "sites", "A single configuration can reach all selected sites", false}, Field{"allow_server_lan", "Allow access to server LAN", "bool", "Default enabled for clients", false}, mode, Field{"dns", "DNS resolvers", "list", "Up to four IPs. In split mode only resolvers inside AllowedIPs are exported", false}, enabled)
	}
	if edit && kind != "groups" {
		fs = append(fs, rotate)
	}
	return fs
}
func Defaults(kind string) Record {
	return Record{"enabled": true, "allow_all_subnets": true, "allow_server_lan": true, "tunnel_mode": "split", "group": "default"}
}

type Page struct {
	Title, Kind, CSRF, User, Error, Notice, Query, ID string
	Rows, Sites, Groups                               []Record
	Item, Status                                      Record
	Fields                                            []Field
	Edit, Form                                        bool
	Output                                            string
}

func Title(k string) string {
	switch k {
	case "sites":
		return "VPN Hub · Sites"
	case "groups":
		return "Client groups"
	case "clients":
		return "VPN Clients"
	case "system":
		return "Engine & maintenance"
	default:
		return "Dashboard"
	}
}
