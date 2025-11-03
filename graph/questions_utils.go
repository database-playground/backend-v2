package graph

import (
	"context"
	"slices"
	"strings"

	"github.com/database-playground/backend-v2/ent"
	entQuestion "github.com/database-playground/backend-v2/ent/question"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/internal/auth"
)

// checkQuestionVisibleScope checks if the user has permission to access the question based on visible_scope.
// Returns nil if the user has access, or an error (ErrNotFound) if they don't.
func checkQuestionVisibleScope(ctx context.Context, question *ent.Question) error {
	visibleScope := question.VisibleScope
	// If visible_scope is empty, the question is visible to everyone
	if strings.TrimSpace(visibleScope) == "" {
		return nil
	}

	// Get user from context
	tokenInfo, ok := auth.GetUser(ctx)
	if !ok {
		// If no user context, but question has visible_scope, return not found
		return defs.ErrNotFound
	}

	// Check if user has the required scope
	for _, scope := range tokenInfo.Scopes {
		if scope == "*" || scope == visibleScope {
			return nil
		}
	}

	return defs.ErrNotFound
}

// applyQuestionVisibleScopeFilter applies visible_scope filtering to a question query.
// If the user has wildcard scope "*", no filtering is applied.
// Otherwise, only questions with nil visible_scope or visible_scope matching user's scopes are included.
func applyQuestionVisibleScopeFilter(ctx context.Context, query *ent.QuestionQuery) *ent.QuestionQuery {
	tokenInfo, ok := auth.GetUser(ctx)
	if !ok {
		// If no user context, only show questions without visible_scope
		return query.Where(entQuestion.VisibleScopeIsNil())
	}

	// If user has full access, don't filter
	if slices.Contains(tokenInfo.Scopes, "*") {
		return query
	}

	// Filter to show only questions with nil visible_scope or visible_scope matching user's scopes
	return query.Where(
		entQuestion.Or(
			entQuestion.VisibleScopeIsNil(),
			entQuestion.VisibleScopeIn(tokenInfo.Scopes...),
		),
	)
}
