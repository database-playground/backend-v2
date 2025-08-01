// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/database-playground/backend-v2/ent/predicate"
	"github.com/database-playground/backend-v2/ent/scopeset"
)

// ScopeSetDelete is the builder for deleting a ScopeSet entity.
type ScopeSetDelete struct {
	config
	hooks    []Hook
	mutation *ScopeSetMutation
}

// Where appends a list predicates to the ScopeSetDelete builder.
func (ssd *ScopeSetDelete) Where(ps ...predicate.ScopeSet) *ScopeSetDelete {
	ssd.mutation.Where(ps...)
	return ssd
}

// Exec executes the deletion query and returns how many vertices were deleted.
func (ssd *ScopeSetDelete) Exec(ctx context.Context) (int, error) {
	return withHooks(ctx, ssd.sqlExec, ssd.mutation, ssd.hooks)
}

// ExecX is like Exec, but panics if an error occurs.
func (ssd *ScopeSetDelete) ExecX(ctx context.Context) int {
	n, err := ssd.Exec(ctx)
	if err != nil {
		panic(err)
	}
	return n
}

func (ssd *ScopeSetDelete) sqlExec(ctx context.Context) (int, error) {
	_spec := sqlgraph.NewDeleteSpec(scopeset.Table, sqlgraph.NewFieldSpec(scopeset.FieldID, field.TypeInt))
	if ps := ssd.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	affected, err := sqlgraph.DeleteNodes(ctx, ssd.driver, _spec)
	if err != nil && sqlgraph.IsConstraintError(err) {
		err = &ConstraintError{msg: err.Error(), wrap: err}
	}
	ssd.mutation.done = true
	return affected, err
}

// ScopeSetDeleteOne is the builder for deleting a single ScopeSet entity.
type ScopeSetDeleteOne struct {
	ssd *ScopeSetDelete
}

// Where appends a list predicates to the ScopeSetDelete builder.
func (ssdo *ScopeSetDeleteOne) Where(ps ...predicate.ScopeSet) *ScopeSetDeleteOne {
	ssdo.ssd.mutation.Where(ps...)
	return ssdo
}

// Exec executes the deletion query.
func (ssdo *ScopeSetDeleteOne) Exec(ctx context.Context) error {
	n, err := ssdo.ssd.Exec(ctx)
	switch {
	case err != nil:
		return err
	case n == 0:
		return &NotFoundError{scopeset.Label}
	default:
		return nil
	}
}

// ExecX is like Exec, but panics if an error occurs.
func (ssdo *ScopeSetDeleteOne) ExecX(ctx context.Context) {
	if err := ssdo.Exec(ctx); err != nil {
		panic(err)
	}
}
