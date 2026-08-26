//go:build windows

package ipc

import (
	"fmt"
	"os"
	"testing"
)

func testEndpoint(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\LOCAL\skk-popup-test-%d`, os.Getpid())
}

func TestEndpointUsesMSIXLocalNamespace(t *testing.T) {
	if Endpoint() != `\\.\pipe\LOCAL\skk-popup` {
		t.Fatalf("Endpoint() = %q", Endpoint())
	}
}
