package cli

import "testing"

func TestLeaseStatusStateCanBeReadyRequiresActiveCoordinatorLease(t *testing.T) {
	tests := []struct {
		name  string
		lease LeaseTarget
		state string
		want  bool
	}{
		{
			name:  "coordinator active",
			lease: LeaseTarget{Coordinator: &CoordinatorClient{}},
			state: "active",
			want:  true,
		},
		{
			name:  "coordinator released",
			lease: LeaseTarget{Coordinator: &CoordinatorClient{}},
			state: "released",
		},
		{
			name:  "coordinator expired",
			lease: LeaseTarget{Coordinator: &CoordinatorClient{}},
			state: "expired",
		},
		{
			name:  "direct ready",
			lease: LeaseTarget{},
			state: "ready",
			want:  true,
		},
		{
			name:  "direct provisioning",
			lease: LeaseTarget{},
			state: "provisioning",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := leaseStatusStateCanBeReady(tt.lease, tt.state); got != tt.want {
				t.Fatalf("leaseStatusStateCanBeReady(%q)=%t, want %t", tt.state, got, tt.want)
			}
		})
	}
}
