package models

import "testing"

func TestIsValidNetworkInterfaceSelector(t *testing.T) {
	tests := []struct {
		selector string
		want     bool
	}{
		{selector: "", want: true},
		{selector: "all", want: true},
		{selector: "net0", want: true},
		{selector: "net12", want: true},
		{selector: "vmbr0", want: true},
		{selector: "br-lan.100", want: true},
		{selector: "vmbr 0", want: false},
		{selector: "vmbr0,net0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			if got := IsValidNetworkInterfaceSelector(tt.selector); got != tt.want {
				t.Fatalf("IsValidNetworkInterfaceSelector(%q) = %v, want %v", tt.selector, got, tt.want)
			}
		})
	}
}
