package workflow

import (
	"reflect"
	"testing"
)

func TestBuildInspectArgoArgs(t *testing.T) {
	args := buildInspectArgoArgs("node-123")

	expected := []string{
		"-n", enrollWorkflowNamespace,
		"submit",
		"--from", inspectWorkflowTemplate,
		"-o", "name",
		"-p", "node=node-123",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args: got %#v want %#v", args, expected)
	}
}
