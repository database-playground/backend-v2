package scope

import (
	"strings"
)

// ShouldAllow checks if a user with given scopes can access a function requiring fnScope
// fnScope format: "resource:action" (e.g. "user:read") or empty for public functions
// userScope format: array of "resource:action", "resource:*", "*:action" or "*" patterns
func ShouldAllow(fnScope string, userScope []string) bool {
	// Public functions are accessible to everyone
	if fnScope == "" {
		return true
	}

	// Check each user scope
	for _, scope := range userScope {
		// Full access scope
		if scope == "*" {
			return true
		}

		// Split function and user scopes into resource and action parts
		fnParts := strings.Split(fnScope, ":")
		scopeParts := strings.Split(scope, ":")

		// Invalid scope format
		if len(fnParts) != 2 || len(scopeParts) != 2 {
			continue
		}

		fnResource, fnAction := fnParts[0], fnParts[1]
		scopeResource, scopeAction := scopeParts[0], scopeParts[1]

		// Check if scope matches function requirements
		resourceMatch := scopeResource == "*" || scopeResource == fnResource
		actionMatch := scopeAction == "*" || scopeAction == fnAction

		if resourceMatch && actionMatch {
			return true
		}
	}

	return false
}
