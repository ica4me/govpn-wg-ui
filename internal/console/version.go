package console

import "os"

// Version is the console release shown in the footer. It is overridden at
// build time with -ldflags "-X govpn-wg-ui/internal/console.Version=...".
var Version = "v1.5.0"

// WireGuardVersion is a deployment-time snapshot, not a per-request probe.
// Set GOVPN_WIREGUARD_VERSION from the backend host's module version.
var WireGuardVersion = wireGuardVersion()

func wireGuardVersion() string {
	if v := os.Getenv("GOVPN_WIREGUARD_VERSION"); v != "" {
		return v
	}
	return "version not configured"
}
