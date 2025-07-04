package schema

import (
	"entgo.io/contrib/entgql"
	"github.com/vektah/gqlparser/v2/ast"
)

// ScopeDirective is a directive that can be used to restrict the scope of a field or mutation.
// It takes a scope string in the format of "resource:action" (e.g. "user:read").
//
// For stuff without this directive, it will be accessible to everyone.
func ScopeDirective(scope string) entgql.Directive {
	return entgql.Directive{
		Name: "scope",
		Arguments: []*ast.Argument{
			{
				Name: "scope",
				Value: &ast.Value{
					Raw:  scope,
					Kind: ast.StringValue,
				},
			},
		},
	}
}
