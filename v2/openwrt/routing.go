package openwrt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// DeviceRouting represents a LAN device with its routing mode.
type DeviceRouting struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
	Mode     string `json:"mode"` // proxy, direct, block
}

// GetDeviceRoutings reads all per-device routing entries from UCI config.
func GetDeviceRoutings() ([]DeviceRouting, error) {
	devices, err := GetLanDevices()
	if err != nil {
		return nil, err
	}

	// Enrich with UCI routing modes
	for i := range devices {
		key := fmt.Sprintf("hiddify.dev_%s.mode", strings.ReplaceAll(devices[i].IP, ".", "_"))
		mode, err := UciGet(key)
		if err == nil && mode != "" {
			devices[i].Mode = mode
		}
	}

	result := make([]DeviceRouting, len(devices))
	for i, d := range devices {
		result[i] = DeviceRouting{
			IP:       d.IP,
			MAC:      d.MAC,
			Hostname: d.Hostname,
			Mode:     d.Mode,
		}
	}
	return result, nil
}

// SetDeviceRouting sets the routing mode for a specific device in UCI.
func SetDeviceRouting(ip, mac, mode string) error {
	section := "dev_" + strings.ReplaceAll(ip, ".", "_")

	cmds := [][]string{
		{"uci", "set", fmt.Sprintf("hiddify.%s=device", section)},
		{"uci", "set", fmt.Sprintf("hiddify.%s.ip=%s", section, ip)},
		{"uci", "set", fmt.Sprintf("hiddify.%s.mac=%s", section, mac)},
		{"uci", "set", fmt.Sprintf("hiddify.%s.mode=%s", section, mode)},
		{"uci", "commit", "hiddify"},
	}

	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("failed: %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// ApplyDeviceRouting updates nftables bypass set from current UCI config.
func ApplyDeviceRouting() error {
	cmd := exec.Command("sh", "-c", ". /usr/lib/hiddify/nftables.sh && hiddify_nft_bypass")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nftables bypass update failed: %s: %w", string(out), err)
	}
	return nil
}

// DeviceRoutingToRules converts per-device routing config to hiddify-core routing rules JSON.
func DeviceRoutingToRules(devices []DeviceRouting) []map[string]interface{} {
	var rules []map[string]interface{}
	for _, d := range devices {
		if d.Mode == "proxy" {
			continue // proxy is default, no explicit rule needed
		}
		rules = append(rules, map[string]interface{}{
			"name":            fmt.Sprintf("%s (%s)", d.Hostname, d.IP),
			"outbound":        d.Mode,
			"source-ip-cidrs": []string{d.IP + "/32"},
			"enabled":         true,
		})
	}
	return rules
}

// DeviceRoutingToJSON converts device routing rules to JSON bytes.
func DeviceRoutingToJSON(devices []DeviceRouting) ([]byte, error) {
	rules := DeviceRoutingToRules(devices)
	return json.Marshal(rules)
}
