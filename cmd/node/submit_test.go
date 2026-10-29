package node

import "testing"

func TestWorkflowUIURL(t *testing.T) {
	tests := []struct {
		server string
		want   string
		ok     bool
	}{
		{"https://kubernetes.dc.example.com:6443", "https://workflows.dc.example.com/workflows/argo-events/enroll-server-abc", true},
		{"https://k8s.example.com", "https://workflows.example.com/workflows/argo-events/enroll-server-abc", true},
		{"https://10.0.0.1:6443", "", false},
		{"https://[fd00::1]:6443", "", false},
		{"https://localhost:6443", "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		got, ok := workflowUIURL(tc.server, "argo-events", "enroll-server-abc")
		if got != tc.want || ok != tc.ok {
			t.Errorf("workflowUIURL(%q) = %q, %v; want %q, %v", tc.server, got, ok, tc.want, tc.ok)
		}
	}
}
