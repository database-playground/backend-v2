package scope_test

import (
	"testing"

	"github.com/database-playground/backend-v2/internal/scope"
)

func TestShouldAllow(t *testing.T) {
	t.Run("users without scope should be able to access public function", func(t *testing.T) {
		if !scope.ShouldAllow("", []string{}) {
			t.Fatal("public function should be allowed")
		}
	})

	t.Run("users with user resource read scope should be able to access public function", func(t *testing.T) {
		if !scope.ShouldAllow("", []string{"user:read"}) {
			t.Fatal("public function should be allowed")
		}
	})

	t.Run("users with all user resource scope should be able to access public function", func(t *testing.T) {
		if !scope.ShouldAllow("", []string{"user:*"}) {
			t.Fatal("public function should be allowed")
		}
	})

	t.Run("users with any scope should be able to access public function", func(t *testing.T) {
		if !scope.ShouldAllow("", []string{"*"}) {
			t.Fatal("public function should be allowed")
		}
	})

	t.Run("users with no scope should not be able to access private function", func(t *testing.T) {
		if scope.ShouldAllow("user:read", []string{}) {
			t.Fatal("private function should not be allowed")
		}
	})

	t.Run("users with any scope should be able to access private function", func(t *testing.T) {
		if !scope.ShouldAllow("user:read", []string{"*"}) {
			t.Fatal("private function should be allowed")
		}
	})

	t.Run("users with user resource read scope should be able to access user read function", func(t *testing.T) {
		userScope := []string{"user:read"}

		if !scope.ShouldAllow("user:read", userScope) {
			t.Fatal("private function [user:read] should be allowed")
		}

		if scope.ShouldAllow("user:write", userScope) {
			t.Fatal("private function [user:write] should not be allowed")
		}

		if scope.ShouldAllow("question:read", userScope) {
			t.Fatal("private function [question:read] should not be allowed")
		}

		if scope.ShouldAllow("question:write", userScope) {
			t.Fatal("private function [question:write] should not be allowed")
		}
	})

	t.Run("users with all user resource scope should be able to access user function", func(t *testing.T) {
		userScope := []string{"user:*"}

		if !scope.ShouldAllow("user:read", userScope) {
			t.Fatal("private function [user:read] should be allowed")
		}

		if !scope.ShouldAllow("user:write", userScope) {
			t.Fatal("private function [user:write] should be allowed")
		}

		if scope.ShouldAllow("question:read", userScope) {
			t.Fatal("private function [question:read] should not be allowed")
		}

		if scope.ShouldAllow("question:write", userScope) {
			t.Fatal("private function [question:write] should not be allowed")
		}
	})

	t.Run("users with all resource read scope should be able to access read function", func(t *testing.T) {
		userScope := []string{"*:read"}

		if !scope.ShouldAllow("user:read", userScope) {
			t.Fatal("private function [user:read] should be allowed")
		}

		if !scope.ShouldAllow("question:read", userScope) {
			t.Fatal("private function [question:read] should be allowed")
		}

		if scope.ShouldAllow("user:write", userScope) {
			t.Fatal("private function [user:write] should not be allowed")
		}

		if scope.ShouldAllow("question:write", userScope) {
			t.Fatal("private function [question:write] should not be allowed")
		}
	})

	t.Run("users with many scopes should be able to access certain function", func(t *testing.T) {
		userScope := []string{"question:write", "user:read"}

		if !scope.ShouldAllow("user:read", userScope) {
			t.Fatal("private function [user:read] should be allowed")
		}

		if !scope.ShouldAllow("question:write", userScope) {
			t.Fatal("private function [question:write] should be allowed")
		}

		if scope.ShouldAllow("user:write", userScope) {
			t.Fatal("private function [user:write] should not be allowed")
		}

		if scope.ShouldAllow("question:read", userScope) {
			t.Fatal("private function [question:read] should not be allowed")
		}
	})

	t.Run("users with bad scope format should not be able to access any function", func(t *testing.T) {
		userScope := []string{"user:read:write"}

		if scope.ShouldAllow("user:read", userScope) {
			t.Fatal("private function [user:read] should not be allowed")
		}
	})
}
