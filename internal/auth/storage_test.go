package auth_test

import (
	"errors"
	"testing"

	"github.com/database-playground/backend-v2/internal/auth"
)

func TestTokenInfo_Validate(t *testing.T) {
	t.Run("valid token info", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}

		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("user id is zero", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    0,
			UserEmail: "test@example.com",
			Machine:   "test",
		}

		err := tokenInfo.Validate()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrValidationPositiveUserID) {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("user id is negative", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    -1,
			UserEmail: "test@example.com",
			Machine:   "test",
		}

		err := tokenInfo.Validate()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrValidationPositiveUserID) {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("user email is empty", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "",
			Machine:   "test",
		}

		err := tokenInfo.Validate()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrValidationRequireUserEmail) {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("machine is empty", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "",
		}

		err := tokenInfo.Validate()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrValidationRequireMachine) {
			t.Fatalf("expected error, got %v", err)
		}
	})

	t.Run("scopes is empty", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
		}

		err := tokenInfo.Validate()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}

		if !errors.Is(err, auth.ErrValidationAtLeastOneScope) {
			t.Fatalf("expected error, got %v", err)
		}
	})
}

func TestTokenInfo_Validate_EdgeCases(t *testing.T) {
	t.Run("zero value token info", func(t *testing.T) {
		var tokenInfo auth.TokenInfo
		err := tokenInfo.Validate()
		if err == nil {
			t.Fatal("expected error for zero value token info")
		}
		if !errors.Is(err, auth.ErrValidationPositiveUserID) {
			t.Fatalf("expected ErrValidationPositiveUserID, got %v", err)
		}
	})

	t.Run("user email with whitespace", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "  test@example.com  ",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for valid email with whitespace, got %v", err)
		}
	})

	t.Run("machine with whitespace", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "  test-machine  ",
			Scopes:    []string{"*"},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for valid machine with whitespace, got %v", err)
		}
	})

	t.Run("empty scopes slice", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{},
		}
		err := tokenInfo.Validate()
		if err == nil {
			t.Fatal("expected error for empty scopes slice")
		}
		if !errors.Is(err, auth.ErrValidationAtLeastOneScope) {
			t.Fatalf("expected ErrValidationAtLeastOneScope, got %v", err)
		}
	})

	t.Run("nil scopes slice", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    nil,
		}
		err := tokenInfo.Validate()
		if err == nil {
			t.Fatal("expected error for nil scopes slice")
		}
		if !errors.Is(err, auth.ErrValidationAtLeastOneScope) {
			t.Fatalf("expected ErrValidationAtLeastOneScope, got %v", err)
		}
	})

	t.Run("scopes with empty strings", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"", "read", ""},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for scopes with empty strings, got %v", err)
		}
	})

	t.Run("meta field is optional", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
			Meta:      nil,
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for nil meta, got %v", err)
		}
	})

	t.Run("meta field with values", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
			Meta: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for meta with values, got %v", err)
		}
	})
}

func TestTokenInfo_Validate_MultipleErrors(t *testing.T) {
	t.Run("multiple validation errors", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    0,   // invalid
			UserEmail: "",  // invalid
			Machine:   "",  // invalid
			Scopes:    nil, // invalid
		}
		err := tokenInfo.Validate()
		if err == nil {
			t.Fatal("expected error for multiple validation failures")
		}
		// Should return the first validation error encountered
		if !errors.Is(err, auth.ErrValidationPositiveUserID) {
			t.Fatalf("expected ErrValidationPositiveUserID, got %v", err)
		}
	})
}

func TestTokenInfo_Validate_BoundaryValues(t *testing.T) {
	t.Run("user ID at boundary", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    1, // minimum valid value
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for user ID 1, got %v", err)
		}
	})

	t.Run("user ID just below boundary", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    0, // just below minimum
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		err := tokenInfo.Validate()
		if err == nil {
			t.Fatal("expected error for user ID 0")
		}
		if !errors.Is(err, auth.ErrValidationPositiveUserID) {
			t.Fatalf("expected ErrValidationPositiveUserID, got %v", err)
		}
	})

	t.Run("large user ID", func(t *testing.T) {
		tokenInfo := auth.TokenInfo{
			UserID:    999999999,
			UserEmail: "test@example.com",
			Machine:   "test",
			Scopes:    []string{"*"},
		}
		err := tokenInfo.Validate()
		if err != nil {
			t.Fatalf("expected no error for large user ID, got %v", err)
		}
	})
}
