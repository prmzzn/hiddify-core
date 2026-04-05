package openwrt

import (
	"encoding/json"
	"testing"
)

func TestDeviceRoutingToRules_SkipsProxy(t *testing.T) {
	devices := []DeviceRouting{
		{IP: "192.168.1.100", Hostname: "Phone", Mode: "proxy"},
		{IP: "192.168.1.101", Hostname: "Laptop", Mode: "direct"},
		{IP: "192.168.1.102", Hostname: "TV", Mode: "block"},
	}
	rules := DeviceRoutingToRules(devices)

	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (skip proxy), got %d", len(rules))
	}
	if rules[0]["outbound"] != "direct" {
		t.Errorf("expected direct, got %v", rules[0]["outbound"])
	}
	if rules[1]["outbound"] != "block" {
		t.Errorf("expected block, got %v", rules[1]["outbound"])
	}
}

func TestDeviceRoutingToRules_SourceCIDR(t *testing.T) {
	devices := []DeviceRouting{
		{IP: "192.168.1.50", Hostname: "PC", Mode: "direct"},
	}
	rules := DeviceRoutingToRules(devices)

	cidrs, ok := rules[0]["source-ip-cidrs"].([]string)
	if !ok || len(cidrs) != 1 || cidrs[0] != "192.168.1.50/32" {
		t.Errorf("expected source-ip-cidrs [192.168.1.50/32], got %v", rules[0]["source-ip-cidrs"])
	}
}

func TestDeviceRoutingToJSON(t *testing.T) {
	devices := []DeviceRouting{
		{IP: "10.0.0.1", Hostname: "Server", Mode: "direct"},
	}
	data, err := DeviceRoutingToJSON(devices)
	if err != nil {
		t.Fatal(err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(parsed))
	}
	if parsed[0]["outbound"] != "direct" {
		t.Errorf("expected direct, got %v", parsed[0]["outbound"])
	}
}

func TestDeviceRoutingToRules_Empty(t *testing.T) {
	rules := DeviceRoutingToRules(nil)
	if rules != nil {
		t.Errorf("expected nil for empty devices, got %v", rules)
	}
}
