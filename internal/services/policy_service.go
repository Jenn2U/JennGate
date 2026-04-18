package services

import (
	"fmt"
	"sync"
)

// Policy represents the access permissions for a user on a device.
type Policy struct {
	Permissions []string
}

// PolicyService manages RBAC policies for user-device access control.
// It uses an in-memory cache with RWMutex for thread-safe concurrent access.
type PolicyService struct {
	cache map[string]*Policy
	mu    sync.RWMutex
}

// NewPolicyService creates a new PolicyService with an empty cache.
func NewPolicyService() *PolicyService {
	return &PolicyService{
		cache: make(map[string]*Policy),
	}
}

// CanAccessGUI checks if a user has gate.gui.access permission for a device.
// Returns false if the policy doesn't exist or the permission is missing.
func (ps *PolicyService) CanAccessGUI(userID, deviceID string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, deviceID)
	policy, exists := ps.cache[key]
	if !exists {
		return false
	}

	for _, perm := range policy.Permissions {
		if perm == "gate.gui.access" {
			return true
		}
	}

	return false
}

// SetPolicy sets the permissions policy for a user on a device.
// Used for testing and policy synchronization.
func (ps *PolicyService) SetPolicy(userID, deviceID string, permissions []string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, deviceID)
	ps.cache[key] = &Policy{
		Permissions: permissions,
	}
}
