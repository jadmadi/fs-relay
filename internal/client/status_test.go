package client

import (
	"strings"
	"testing"
)

func TestStatusErrMissingNamespace(t *testing.T) {
	err := statusErr("join", 404, []byte("namespace demo does not exist"))
	if !Missing(err) {
		t.Fatalf("not missing: %v", err)
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("message %q", err.Error())
	}
}
