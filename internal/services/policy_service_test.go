package services

import (
	"testing"
)

// TestCanAccessGUI tests the CanAccessGUI method for RBAC gate.gui.access check.
func TestCanAccessGUI(t *testing.T) {
	ps := NewPolicyService()

	// Add policy: user has gate.connect and gate.gui.access for device
	ps.SetPolicy("user-123", "device-456",
		[]string{"gate.connect", "gate.gui.access"})

	// Test: user can access GUI
	canAccess := ps.CanAccessGUI("user-123", "device-456")
	if !canAccess {
		t.Fatal("expected CanAccessGUI to return true for user with permission")
	}

	// Test: user without permission cannot access
	canAccess = ps.CanAccessGUI("user-789", "device-456")
	if canAccess {
		t.Fatal("expected CanAccessGUI to return false for user without policy")
	}

	// Test: user with policy but missing gate.gui.access cannot access
	ps.SetPolicy("user-999", "device-456", []string{"gate.connect"})
	canAccess = ps.CanAccessGUI("user-999", "device-456")
	if canAccess {
		t.Fatal("expected CanAccessGUI to return false for user without gate.gui.access permission")
	}
}

// TestSetPolicy tests the SetPolicy method for setting permissions.
func TestSetPolicy(t *testing.T) {
	ps := NewPolicyService()

	// Set a policy
	permissions := []string{"gate.connect", "gate.gui.access", "gate.recording"}
	ps.SetPolicy("user-123", "device-456", permissions)

	// Verify all permissions are stored
	if !ps.CanAccessGUI("user-123", "device-456") {
		t.Fatal("expected gate.gui.access permission to be set")
	}

	// Update the policy with different permissions
	ps.SetPolicy("user-123", "device-456", []string{"gate.connect"})

	// Verify old permission is gone
	if ps.CanAccessGUI("user-123", "device-456") {
		t.Fatal("expected gate.gui.access permission to be removed after policy update")
	}
}

// TestCanAccessGUIEmptyPermissions tests CanAccessGUI with empty permissions.
func TestCanAccessGUIEmptyPermissions(t *testing.T) {
	ps := NewPolicyService()

	// Set a policy with empty permissions
	ps.SetPolicy("user-123", "device-456", []string{})

	// Should not have gate.gui.access
	canAccess := ps.CanAccessGUI("user-123", "device-456")
	if canAccess {
		t.Fatal("expected CanAccessGUI to return false for empty permissions")
	}
}

// TestCanAccessGUIConcurrency tests that PolicyService is thread-safe.
func TestCanAccessGUIConcurrency(t *testing.T) {
	ps := NewPolicyService()

	// Set initial policy
	ps.SetPolicy("user-1", "device-1", []string{"gate.gui.access"})

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = ps.CanAccessGUI("user-1", "device-1")
			done <- true
		}()
	}

	// Wait for all reads to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent writes and reads
	done = make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			ps.SetPolicy("user-2", "device-2", []string{"gate.gui.access"})
			done <- true
		}()
		go func() {
			_ = ps.CanAccessGUI("user-1", "device-1")
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify final state
	if !ps.CanAccessGUI("user-2", "device-2") {
		t.Fatal("policy should be set after concurrent operations")
	}
}

// TestCanAccessGUIMultipleDevices tests access control across multiple devices.
func TestCanAccessGUIMultipleDevices(t *testing.T) {
	ps := NewPolicyService()

	// User has access to device A
	ps.SetPolicy("user-123", "device-A", []string{"gate.gui.access"})

	// User does not have access to device B
	ps.SetPolicy("user-123", "device-B", []string{"gate.connect"})

	// Verify separate policies
	if !ps.CanAccessGUI("user-123", "device-A") {
		t.Fatal("expected CanAccessGUI to return true for device-A")
	}

	if ps.CanAccessGUI("user-123", "device-B") {
		t.Fatal("expected CanAccessGUI to return false for device-B")
	}
}
