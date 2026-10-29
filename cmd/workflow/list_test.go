package workflow

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildListArgoArgsNoFilters(t *testing.T) {
	got := buildListArgoArgs(&listOptions{})
	want := []string{"-n", enrollWorkflowNamespace, "list", "-o", "json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestBuildListArgoArgsFilters(t *testing.T) {
	got := buildListArgoArgs(&listOptions{
		Templates:  []string{"enroll-server", "enroll-fw"},
		Kind:       "enroll",
		DeviceType: "fw",
		Target:     "fw a",
		Status:     "Failed,Error",
		Selector:   "team=net",
		Output:     "wide",
	})
	want := []string{
		"-n", enrollWorkflowNamespace, "list",
		"-l", "workflows.argoproj.io/workflow-template in (enroll-server,enroll-fw)," +
			"understack.rackspace.com/workflow-kind=enroll," +
			"understack.rackspace.com/device-type=fw," +
			"understack.rackspace.com/target=fw-a," +
			"team=net",
		"--status", "Failed,Error",
		"-o", "wide",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRenderWorkflowTable(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	data := []byte(`[
	  {"metadata":{"name":"old","creationTimestamp":"2026-10-28T09:00:00Z"},
	   "status":{"phase":"Succeeded","startedAt":"2026-10-28T09:00:00Z","finishedAt":"2026-10-28T09:05:00Z"}},
	  {"metadata":{"name":"new","creationTimestamp":"2026-10-28T11:50:00Z"},
	   "status":{"phase":"Running","startedAt":"2026-10-28T11:50:00Z"}},
	  {"metadata":{"name":"pending","creationTimestamp":"2026-10-28T11:59:30Z"},
	   "status":{"phase":"Pending"}}
	]`)

	var out bytes.Buffer
	if err := renderWorkflowTable(&out, data, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), out.String())
	}
	if strings.Contains(out.String(), "PRIORITY") {
		t.Fatalf("PRIORITY column should not be shown:\n%s", out.String())
	}

	for i, want := range [][]string{
		{"NAME", "STATUS", "AGE", "DURATION"},
		{"pending", "Pending", "30s", "0s"},
		{"new", "Running", "10m", "10m"},
		{"old", "Succeeded", "3h", "5m"},
	} {
		if got := strings.Fields(lines[i]); !reflect.DeepEqual(got, want) {
			t.Errorf("line %d: got %v want %v", i, got, want)
		}
	}
}

func TestRenderWorkflowTableEmpty(t *testing.T) {
	for _, in := range []string{"[]", "null"} {
		var out bytes.Buffer
		if err := renderWorkflowTable(&out, []byte(in), time.Now()); err != nil {
			t.Fatalf("%s: unexpected error: %v", in, err)
		}
		if strings.TrimSpace(out.String()) != "No workflows found" {
			t.Errorf("%s: got %q", in, out.String())
		}
	}
}

func TestBuildListSelectorSingleTemplate(t *testing.T) {
	got := buildListSelector(&listOptions{Templates: []string{" enroll-server ", ""}})
	want := "workflows.argoproj.io/workflow-template=enroll-server"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
