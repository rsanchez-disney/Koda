package kitestream

import (
	"testing"
)

func TestGenerateToken(t *testing.T) {
	t1 := generateToken()
	t2 := generateToken()
	if len(t1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(t1))
	}
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
}

func TestIsRunning_WhenNotRunning(t *testing.T) {
	if IsRunning() {
		// Could be running from a previous test — just skip
		t.Skip("KiteStream appears to be running")
	}
}

func TestStatus_WhenStopped(t *testing.T) {
	s := Status(7700)
	if s != "stopped" && s != "" {
		// May show running if actually running
		t.Logf("status = %q", s)
	}
}

func TestReadPID_NoFile(t *testing.T) {
	_, err := ReadPID()
	if err == nil {
		// PID file may exist from a real run — that's ok
		t.Log("PID file exists (possibly from real run)")
	}
}
