package workflow

import (
	"strings"
	"testing"
)

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

func TestSanitizeLabelValue(t *testing.T) {
	tests := map[string]string{
		"10.46.6.165":                  "10.46.6.165",
		"fw-a.dc.example.com":          "fw-a.dc.example.com",
		"Doug Goldstein":               "Doug-Goldstein",
		"corp\\doug":                   "corp-doug",
		"-weird_":                      "weird",
		"":                             "",
		strings.Repeat("a", 70):        strings.Repeat("a", 63),
		strings.Repeat("a", 62) + "-b": strings.Repeat("a", 62),
	}
	for in, want := range tests {
		if got := sanitizeLabelValue(in); got != want {
			t.Errorf("sanitizeLabelValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSubmitLabels(t *testing.T) {
	got := submitLabels("10.46.6.165", "doug")
	want := "understack.rackspace.com/target=10.46.6.165,understack.rackspace.com/submitted-by=doug"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := submitLabels("", ""); got != "" {
		t.Errorf("expected no labels, got %q", got)
	}
}
