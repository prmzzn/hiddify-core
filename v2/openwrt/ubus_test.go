package openwrt

import (
	"encoding/json"
	"testing"
)

func TestStatusResponse_JSON(t *testing.T) {
	s := StatusResponse{
		State:   "RUNNING",
		Uptime:  3600,
		Profile: "my-profile",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["state"] != "RUNNING" {
		t.Errorf("expected RUNNING, got %v", decoded["state"])
	}
	if decoded["uptime"].(float64) != 3600 {
		t.Errorf("expected 3600, got %v", decoded["uptime"])
	}
	if decoded["profile"] != "my-profile" {
		t.Errorf("expected my-profile, got %v", decoded["profile"])
	}
}

func TestConfigGetResponse_JSON(t *testing.T) {
	c := ConfigGetResponse{
		Key:   "web_port",
		Value: "8080",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["key"] != "web_port" {
		t.Errorf("expected web_port, got %v", decoded["key"])
	}
	if decoded["value"] != "8080" {
		t.Errorf("expected 8080, got %v", decoded["value"])
	}
}
