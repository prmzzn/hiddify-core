package openwrt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StatusResponse is returned by ubus call hiddify status.
type StatusResponse struct {
	State   string `json:"state"`
	Uptime  int64  `json:"uptime"`
	Profile string `json:"profile"`
}

// ConfigGetResponse is returned by ubus call hiddify config.get.
type ConfigGetResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// DeviceInfo represents a LAN device for per-device routing.
type DeviceInfo struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
	Mode     string `json:"mode"`
}

// UbusClient calls ubus commands via subprocess.
type UbusClient struct {
	timeout time.Duration
}

// NewUbusClient creates a new ubus client.
func NewUbusClient() *UbusClient {
	return &UbusClient{timeout: 5 * time.Second}
}

// Call executes a ubus call and returns the raw JSON output.
func (u *UbusClient) Call(object, method, params string) ([]byte, error) {
	args := []string{"call", object, method}
	if params != "" {
		args = append(args, params)
	}
	cmd := exec.Command("ubus", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ubus call %s %s failed: %w", object, method, err)
	}
	return out, nil
}

// UciGet reads a UCI config value.
func UciGet(key string) (string, error) {
	cmd := exec.Command("uci", "get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("uci get %s failed: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// UciSet writes a UCI config value and commits.
func UciSet(key, value string) error {
	if err := exec.Command("uci", "set", key+"="+value).Run(); err != nil {
		return fmt.Errorf("uci set %s failed: %w", key, err)
	}
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 0 {
		return fmt.Errorf("invalid UCI key: %s", key)
	}
	if err := exec.Command("uci", "commit", parts[0]).Run(); err != nil {
		return fmt.Errorf("uci commit %s failed: %w", parts[0], err)
	}
	return nil
}

// GetLanDevices reads the ARP table and DHCP leases to list LAN devices.
func GetLanDevices() ([]DeviceInfo, error) {
	cmd := exec.Command("cat", "/proc/net/arp")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var devices []DeviceInfo
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			devices = append(devices, DeviceInfo{
				IP:   fields[0],
				MAC:  fields[3],
				Mode: "proxy", // default
			})
		}
	}

	// Enrich with hostnames from DHCP leases
	leaseOut, err := exec.Command("cat", "/tmp/dhcp.leases").Output()
	if err == nil {
		for _, line := range strings.Split(string(leaseOut), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				mac := fields[1]
				ip := fields[2]
				hostname := fields[3]
				for i := range devices {
					if devices[i].IP == ip || strings.EqualFold(devices[i].MAC, mac) {
						devices[i].Hostname = hostname
					}
				}
			}
		}
	}

	return devices, nil
}

// UbusHandler handles ubus-style requests via HTTP.
type UbusHandler struct {
	statusFn   func() StatusResponse
	ubusClient *UbusClient
}

// NewUbusHandler creates a handler with a status function.
func NewUbusHandler(statusFn func() StatusResponse) *UbusHandler {
	return &UbusHandler{
		statusFn:   statusFn,
		ubusClient: NewUbusClient(),
	}
}

// HandleStatus returns the current hiddify status as JSON.
func (h *UbusHandler) HandleStatus() ([]byte, error) {
	status := h.statusFn()
	return json.Marshal(status)
}

// HandleConfigGet reads a UCI config value.
func (h *UbusHandler) HandleConfigGet(key string) ([]byte, error) {
	value, err := UciGet(key)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ConfigGetResponse{Key: key, Value: value})
}

// HandleConfigSet writes a UCI config value.
func (h *UbusHandler) HandleConfigSet(key, value string) error {
	return UciSet(key, value)
}
