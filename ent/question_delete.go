// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/database-playground/backend-v2/ent/predicate"
	"github.com/database-playground/backend-v2/ent/question"
)

// QuestionDelete is the builder for deleting a Question entity.
type QuestionDelete struct {
	config
	hooks    []Hook
	mutation *QuestionMutation
}

// Where appends a list predicates to the QuestionDelete builder.
func (qd *QuestionDelete) Where(ps ...predicate.Question) *QuestionDelete {
	qd.mutation.Where(ps...)
	return qd
}

// Exec executes the deletion query and returns how many vertices were deleted.
func (qd *QuestionDelete) Exec(ctx context.Context) (int, error) {
	return withHooks(ctx, qd.sqlExec, qd.mutation, qd.hooks)
}

// ExecX is like Exec, but panics if an error occurs.
func (qd *QuestionDelete) ExecX(ctx context.Context) int {
	n, err := qd.Exec(ctx)
	if err != nil {
		panic(err)
	}
	return n
}

func (qd *QuestionDelete) sqlExec(ctx context.Context) (int, error) {
	_spec := sqlgraph.NewDeleteSpec(question.Table, sqlgraph.NewFieldSpec(question.FieldID, field.TypeInt))
	if ps := qd.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	affected, err := sqlgraph.DeleteNodes(ctx, qd.driver, _spec)
	if err != nil && sqlgraph.IsConstraintError(err) {
		err = &ConstraintError{msg: err.Error(), wrap: err}
	}
	qd.mutation.done = true
	return affected, err
}

// QuestionDeleteOne is the builder for deleting a single Question entity.
type QuestionDeleteOne struct {
	qd *QuestionDelete
}

// Where appends a list predicates to the QuestionDelete builder.
func (qdo *QuestionDeleteOne) Where(ps ...predicate.Question) *QuestionDeleteOne {
	qdo.qd.mutation.Where(ps...)
	return qdo
}

// Exec executes the deletion query.
func (qdo *QuestionDeleteOne) Exec(ctx context.Context) error {
	n, err := qdo.qd.Exec(ctx)
	switch {
	case err != nil:
		return err
	case n == 0:
		return &NotFoundError{question.Label}
	default:
		return nil
	}
}

// ExecX is like Exec, but panics if an error occurs.
func (qdo *QuestionDeleteOne) ExecX(ctx context.Context) {
	if err := qdo.Exec(ctx); err != nil {
		panic(err)
	}
}
