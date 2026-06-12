package pve

import (
	"testing"
	"time"
)

func TestParseNetworkRateLimit(t *testing.T) {
	tests := []struct {
		name      string
		netConfig string
		wantRate  float64
		wantFound bool
		wantErr   bool
	}{
		{
			name:      "with rate",
			netConfig: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=2.50,firewall=1",
			wantRate:  2.5,
			wantFound: true,
		},
		{
			name:      "without rate",
			netConfig: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1",
			wantFound: false,
		},
		{
			name:      "invalid rate",
			netConfig: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=bad",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRate, gotFound, err := parseNetworkRateLimit(tt.netConfig)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseNetworkRateLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotFound != tt.wantFound {
				t.Fatalf("found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotRate != tt.wantRate {
				t.Fatalf("rate = %v, want %v", gotRate, tt.wantRate)
			}
		})
	}
}

func TestNetworkRateLimitFromConfig(t *testing.T) {
	rate, err := NetworkRateLimitFromConfig(map[string]interface{}{
		"cores": 2,
		"net0":  "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=1.25",
	})
	if err != nil {
		t.Fatalf("NetworkRateLimitFromConfig() error = %v", err)
	}
	if rate != 1.25 {
		t.Fatalf("rate = %v, want 1.25", rate)
	}

	rate, err = NetworkRateLimitFromConfig(map[string]interface{}{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
	})
	if err != nil {
		t.Fatalf("NetworkRateLimitFromConfig() without rate error = %v", err)
	}
	if rate != 0 {
		t.Fatalf("rate = %v, want 0", rate)
	}

	rate, err = NetworkRateLimitFromConfig(map[string]interface{}{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=1.25",
		"net1": "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1",
	})
	if err != nil {
		t.Fatalf("NetworkRateLimitFromConfig() mixed rates error = %v", err)
	}
	if rate != 0 {
		t.Fatalf("mixed rate = %v, want 0 because net1 is unlimited", rate)
	}

	rate, err = NetworkRateLimitFromConfig(map[string]interface{}{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=1.25",
		"net1": "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1,rate=2.50",
	})
	if err != nil {
		t.Fatalf("NetworkRateLimitFromConfig() multiple rates error = %v", err)
	}
	if rate != 2.5 {
		t.Fatalf("multiple rate = %v, want 2.5", rate)
	}
}

func TestSetNetworkRateLimitInConfig(t *testing.T) {
	netConfig := "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=5.00,firewall=1"

	limited := setNetworkRateLimitInConfig(netConfig, 1.25, true)
	if limited != "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1,rate=1.25" {
		t.Fatalf("limited config = %q", limited)
	}

	unlimited := setNetworkRateLimitInConfig(limited, 0, false)
	if unlimited != "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1" {
		t.Fatalf("unlimited config = %q", unlimited)
	}
}

func TestParseNetworkConfigMAC(t *testing.T) {
	got := parseNetworkConfigMAC("virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1")
	if got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("mac = %q, want aa:bb:cc:dd:ee:ff", got)
	}

	got = parseNetworkConfigMAC("AA:BB:CC:DD:EE:00,bridge=vmbr0")
	if got != "aa:bb:cc:dd:ee:00" {
		t.Fatalf("bare mac = %q, want aa:bb:cc:dd:ee:00", got)
	}
}

func TestSelectedNetworkConfigKeys(t *testing.T) {
	config := map[string]interface{}{
		"net0":    "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
		"net1":    "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1",
		"net2":    "virtio=AA:BB:CC:DD:EE:11,bridge=vmbr0",
		"netstat": "not-a-nic",
	}

	all := selectedNetworkConfigKeys(config, "all")
	if len(all) != 3 || all[0] != "net0" || all[1] != "net1" || all[2] != "net2" {
		t.Fatalf("all keys = %#v, want [net0 net1 net2]", all)
	}

	one := selectedNetworkConfigKeys(config, "net1")
	if len(one) != 1 || one[0] != "net1" {
		t.Fatalf("one key = %#v, want [net1]", one)
	}

	bridge := selectedNetworkConfigKeys(config, "vmbr0")
	if len(bridge) != 2 || bridge[0] != "net0" || bridge[1] != "net2" {
		t.Fatalf("bridge keys = %#v, want [net0 net2]", bridge)
	}
}

func TestParseNetworkBridge(t *testing.T) {
	got := parseNetworkBridge("virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1")
	if got != "vmbr0" {
		t.Fatalf("bridge = %q, want vmbr0", got)
	}
}

func TestCreationTimeFromConfig(t *testing.T) {
	got, err := CreationTimeFromConfig(map[string]interface{}{
		"meta": "creation-qemu=8.1.2, ctime=1767225600",
	})
	if err != nil {
		t.Fatalf("CreationTimeFromConfig() error = %v", err)
	}
	want := time.Unix(1767225600, 0)
	if !got.Equal(want) {
		t.Fatalf("creation time = %s, want %s", got, want)
	}

	if _, err := CreationTimeFromConfig(map[string]interface{}{}); err == nil {
		t.Fatal("CreationTimeFromConfig() without meta should fail")
	}
}

func TestNetworkRateLimitUpdatesForInterfaceOnlyTightens(t *testing.T) {
	config := map[string]interface{}{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=5.00",
		"net1": "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1",
		"net2": "virtio=AA:BB:CC:DD:EE:11,bridge=vmbr1,rate=20.00",
	}

	updates, err := networkRateLimitUpdatesForInterface(config, "all", 10)
	if err != nil {
		t.Fatalf("networkRateLimitUpdatesForInterface() error = %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("updates = %#v, want net1 and net2 only", updates)
	}
	if _, ok := updates["net0"]; ok {
		t.Fatalf("net0 should not be updated because 5MB/s is stricter than 10MB/s")
	}
	if updates["net1"] != "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1,rate=10.00" {
		t.Fatalf("net1 update = %q", updates["net1"])
	}
	if updates["net2"] != "virtio=AA:BB:CC:DD:EE:11,bridge=vmbr1,rate=10.00" {
		t.Fatalf("net2 update = %q", updates["net2"])
	}

	updates, err = networkRateLimitUpdatesForInterface(config, "vmbr1", 10)
	if err != nil {
		t.Fatalf("networkRateLimitUpdatesForInterface(vmbr1) error = %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("bridge updates = %#v, want net1 and net2 only", updates)
	}
	if _, ok := updates["net0"]; ok {
		t.Fatalf("net0 should not be updated because it is not bridged to vmbr1")
	}
}

func TestNetworkLinkDownStatesFromConfig(t *testing.T) {
	states, err := NetworkLinkDownStatesFromConfig(map[string]interface{}{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,link_down=1",
		"net1": "virtio=AA:BB:CC:DD:EE:00,bridge=vmbr1",
	})
	if err != nil {
		t.Fatalf("NetworkLinkDownStatesFromConfig() error = %v", err)
	}
	if !states["net0"] {
		t.Fatalf("net0 link_down = false, want true")
	}
	if states["net1"] {
		t.Fatalf("net1 link_down = true, want false")
	}
}
