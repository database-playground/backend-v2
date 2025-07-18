// Code generated by ent, DO NOT EDIT.

package ent

import (
	"fmt"
	"strings"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/database-playground/backend-v2/ent/database"
)

// Database is the model entity for the Database schema.
type Database struct {
	config `json:"-"`
	// ID of the ent.
	ID int `json:"id,omitempty"`
	// Slug holds the value of the "slug" field.
	Slug string `json:"slug,omitempty"`
	// relation figure
	RelationFigure string `json:"relation_figure,omitempty"`
	// Description holds the value of the "description" field.
	Description string `json:"description,omitempty"`
	// SQL schema
	Schema            string `json:"schema,omitempty"`
	question_database *int
	selectValues      sql.SelectValues
}

// scanValues returns the types for scanning values from sql.Rows.
func (*Database) scanValues(columns []string) ([]any, error) {
	values := make([]any, len(columns))
	for i := range columns {
		switch columns[i] {
		case database.FieldID:
			values[i] = new(sql.NullInt64)
		case database.FieldSlug, database.FieldRelationFigure, database.FieldDescription, database.FieldSchema:
			values[i] = new(sql.NullString)
		case database.ForeignKeys[0]: // question_database
			values[i] = new(sql.NullInt64)
		default:
			values[i] = new(sql.UnknownType)
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the Database fields.
func (d *Database) assignValues(columns []string, values []any) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case database.FieldID:
			value, ok := values[i].(*sql.NullInt64)
			if !ok {
				return fmt.Errorf("unexpected type %T for field id", value)
			}
			d.ID = int(value.Int64)
		case database.FieldSlug:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field slug", values[i])
			} else if value.Valid {
				d.Slug = value.String
			}
		case database.FieldRelationFigure:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field relation_figure", values[i])
			} else if value.Valid {
				d.RelationFigure = value.String
			}
		case database.FieldDescription:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field description", values[i])
			} else if value.Valid {
				d.Description = value.String
			}
		case database.FieldSchema:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field schema", values[i])
			} else if value.Valid {
				d.Schema = value.String
			}
		case database.ForeignKeys[0]:
			if value, ok := values[i].(*sql.NullInt64); !ok {
				return fmt.Errorf("unexpected type %T for edge-field question_database", value)
			} else if value.Valid {
				d.question_database = new(int)
				*d.question_database = int(value.Int64)
			}
		default:
			d.selectValues.Set(columns[i], values[i])
		}
	}
	return nil
}

// Value returns the ent.Value that was dynamically selected and assigned to the Database.
// This includes values selected through modifiers, order, etc.
func (d *Database) Value(name string) (ent.Value, error) {
	return d.selectValues.Get(name)
}

// Update returns a builder for updating this Database.
// Note that you need to call Database.Unwrap() before calling this method if this Database
// was returned from a transaction, and the transaction was committed or rolled back.
func (d *Database) Update() *DatabaseUpdateOne {
	return NewDatabaseClient(d.config).UpdateOne(d)
}

// Unwrap unwraps the Database entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (d *Database) Unwrap() *Database {
	_tx, ok := d.config.driver.(*txDriver)
	if !ok {
		panic("ent: Database is not a transactional entity")
	}
	d.config.driver = _tx.drv
	return d
}

// String implements the fmt.Stringer.
func (d *Database) String() string {
	var builder strings.Builder
	builder.WriteString("Database(")
	builder.WriteString(fmt.Sprintf("id=%v, ", d.ID))
	builder.WriteString("slug=")
	builder.WriteString(d.Slug)
	builder.WriteString(", ")
	builder.WriteString("relation_figure=")
	builder.WriteString(d.RelationFigure)
	builder.WriteString(", ")
	builder.WriteString("description=")
	builder.WriteString(d.Description)
	builder.WriteString(", ")
	builder.WriteString("schema=")
	builder.WriteString(d.Schema)
	builder.WriteByte(')')
	return builder.String()
}

// Databases is a parsable slice of Database.
type Databases []*Database
