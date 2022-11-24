package column

import (
	"fmt"
	"time"
)

type ColumnType int

const (
	ColumnTypeText ColumnType = iota
	ColumnTypeInt
	ColumnTypeBigInt
	ColumnTypeBool
	ColumnTypeTimestamp
	ColumnTypeJSONB
	ColumnTypeUUID
)

const NoDefault = "PANIC"
const ArrayJSONDefault = "{}"

type Transform func(record map[string]any, col any) any

type Constraints struct {
	Unique     bool
	ForeignKey [2]string
}

func (c *Constraints) SetUnique(b bool) *Constraints {
	c.Unique = b
	return c
}

func (c *Constraints) SetForeignKey(fk [2]string) *Constraints {
	c.ForeignKey = fk
	return c
}

type RawConstraint struct {
	Type string
	SQL  func(dstName string) string
}

func (c *Constraints) Raw() []RawConstraint {
	constraints := []RawConstraint{}

	if c.Unique {
		constraints = append(constraints, RawConstraint{Type: "unique", SQL: func(dstName string) string {
			return "UNIQUE (" + dstName + ")"
		}})
	}

	if len(c.ForeignKey) > 0 && c.ForeignKey[0] != "" && c.ForeignKey[1] != "" {
		constraints = append(constraints, RawConstraint{
			Type: "fk",
			SQL: func(dstName string) string {
				return "FOREIGN KEY (" + dstName + ") REFERENCES " + c.ForeignKey[0] + "(" + c.ForeignKey[1] + ") ON DELETE CASCADE ON UPDATE CASCADE"
			},
		})
	}

	return constraints
}

type Column struct {
	// The underlying type of the column.
	Type ColumnType
	// Whether the column is an array or not.
	Array bool
	// Nullable?
	Nullable bool
	// Any constraints
	Constraints *Constraints
	// The source name of the column.
	SrcName string
	// The output name of the column.
	DstName string
	// Default value of the column.
	Default any
	// SQL default value of the column.
	SQLDefault string
	// Any transformations for the column.
	Transforms []Transform
}

func (c *Column) GetDefault() string {
	if c.Default != nil {
		switch casted := c.Default.(type) {
		case string:
			return fmt.Sprintf("'%v'", casted)
		case time.Time:
			return fmt.Sprintf("'%v'", casted.Format(time.RFC3339))
		default:
			return fmt.Sprintf("%v", casted)
		}
	}

	if c.SQLDefault != "" {
		return c.SQLDefault
	}

	return ""
}

func (c *Column) BaseType() string {
	switch c.Type {
	case ColumnTypeText:
		return "text"
	case ColumnTypeInt:
		return "int"
	case ColumnTypeBigInt:
		return "bigint"
	case ColumnTypeBool:
		return "bool"
	case ColumnTypeTimestamp:
		return "timestamptz"
	case ColumnTypeJSONB:
		return "jsonb"
	case ColumnTypeUUID:
		return "uuid"
	}

	panic("unknown column type")
}

func (c *Column) SQLType() string {
	if c.Array {
		return c.BaseType() + "[]"
	} else {
		return c.BaseType()
	}
}

func (c *Column) Meta() []string {
	var meta []string
	if !c.Nullable {
		meta = append(meta, "NOT NULL")
	}

	if (c.Default != nil && c.Default != "SKIP") || c.SQLDefault != "" {
		meta = append(meta, "DEFAULT "+c.GetDefault())
	}

	return meta
}

func (c *Column) SetArray(b bool) *Column {
	c.Array = b
	return c
}

func (c *Column) SetConstraints(cn Constraints) *Column {
	c.Constraints = &cn
	return c
}

func (c *Column) SetNullable(b bool) *Column {
	c.Nullable = b
	return c
}

func (c *Column) SetUnique(b bool) *Column {
	c.Constraints.SetUnique(b)
	return c
}

func (c *Column) SetForeignKey(fk [2]string) *Column {
	c.Constraints.SetForeignKey(fk)
	return c
}

func (c *Column) SetSQLDefault(defValue string) *Column {
	c.SQLDefault = defValue
	return c
}

func NewText(srcName, dstName string, defValue any, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeText,
		SrcName:     srcName,
		DstName:     dstName,
		Default:     defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewInt(srcName, dstName string, defValue any, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeInt,
		SrcName:     srcName,
		DstName:     dstName,
		Default:     defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewBigInt(srcName, dstName string, defValue any, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeBigInt,
		SrcName:     srcName,
		DstName:     dstName,
		Default:     defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewBool(srcName, dstName string, defValue bool, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeBool,
		SrcName:     srcName,
		DstName:     dstName,
		Default:     defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewTimestamp(srcName, dstName string, defValue string, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeTimestamp,
		SrcName:     srcName,
		DstName:     dstName,
		SQLDefault:  defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewJSONB(srcName, dstName string, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeJSONB,
		SrcName:     srcName,
		DstName:     dstName,
		Default:     "{}",
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

func NewUUID(srcName, dstName string, defValue string, transforms ...Transform) *Column {
	return &Column{
		Type:        ColumnTypeUUID,
		SrcName:     srcName,
		DstName:     dstName,
		SQLDefault:  defValue,
		Transforms:  transforms,
		Constraints: &Constraints{},
	}
}

// To make things more ergonomic
func Columns(cols ...*Column) []*Column {
	return cols
}

// Explicit declares
func Source(s string) string {
	return s
}

func Dest(s string) string {
	return s
}

func Default[T any](s T) T {
	return s
}
