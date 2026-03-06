package api

import "testing"

func TestApplyIPFallback(t *testing.T) {
	tests := []struct {
		name       string
		vm         *VM
		originalIP string
		wantIP     string
	}{
		{
			name:       "sets original IP when refreshed VM has no IP",
			vm:         &VM{IP: ""},
			originalIP: "192.168.10.25",
			wantIP:     "192.168.10.25",
		},
		{
			name:       "keeps refreshed runtime IP",
			vm:         &VM{IP: "10.20.30.40"},
			originalIP: "192.168.10.25",
			wantIP:     "10.20.30.40",
		},
		{
			name:       "keeps empty when no original IP exists",
			vm:         &VM{IP: ""},
			originalIP: "",
			wantIP:     "",
		},
		{
			name:       "nil VM is a no-op",
			vm:         nil,
			originalIP: "192.168.10.25",
			wantIP:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyIPFallback(tt.vm, tt.originalIP)

			if tt.vm == nil {
				return
			}

			if tt.vm.IP != tt.wantIP {
				t.Fatalf("unexpected IP: got %q want %q", tt.vm.IP, tt.wantIP)
			}
		})
	}
}
