package gosmo

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// -- User-defined functions -----------------------------------------------------

// UserDefinedFunction represents a UDF.
type UserDefinedFunction struct {
	db *Database

	ObjectID   int
	Schema     string
	Name       string
	FuncType   FunctionType
	CreateDate time.Time
	ModifyDate time.Time
}

// FunctionType is a function's sys.objects type: T-SQL, CLR, or a Fabric
// external function. A CLR aggregate (AF) is not one — CREATE AGGREGATE makes
// it and DROP AGGREGATE removes it.
type FunctionType string

const (
	FunctionTypeScalar      FunctionType = "FN" // T-SQL scalar
	FunctionTypeInlineTable FunctionType = "IF" // T-SQL inline table-valued
	FunctionTypeTable       FunctionType = "TF" // T-SQL multi-statement table-valued
	FunctionTypeCLRScalar   FunctionType = "FS" // CLR scalar
	FunctionTypeCLRTable    FunctionType = "FT" // CLR table-valued
	FunctionTypeExternal    FunctionType = "XF" // Fabric external scalar
)

// IsScalar reports a function that returns a value — the one
// a call selects rather than selects from.
func (t FunctionType) IsScalar() bool {
	return t == FunctionTypeScalar || t == FunctionTypeCLRScalar || t == FunctionTypeExternal
}

// IsCLR reports a function implemented in an assembly, with no T-SQL
// definition.
func (t FunctionType) IsCLR() bool {
	return t == FunctionTypeCLRScalar || t == FunctionTypeCLRTable
}

// UserDefinedFunctions returns all UDFs in the database, T-SQL and CLR.
func (d *Database) UserDefinedFunctions(ctx context.Context) ([]*UserDefinedFunction, error) {
	return d.UserDefinedFunctionsFiltered(ctx, ObjectFilter{})
}

// UserDefinedFunctionsFiltered returns the UDFs an ObjectFilter matches,
// narrowed by the server. An empty filter is UserDefinedFunctions.
func (d *Database) UserDefinedFunctionsFiltered(ctx context.Context, filter ObjectFilter) ([]*UserDefinedFunction, error) {
	where, args := filter.clause(allObjectsFilterColumns, 1)
	return d.functionsWhere(ctx, "o.type IN ('FN','TF','IF','FS','FT') AND o.is_ms_shipped = 0", where, args)
}

// ExternalFunctions returns Fabric external functions (sys.objects type XF).
// These are separate from the T-SQL and CLR UserDefinedFunctions listing.
func (d *Database) ExternalFunctions(ctx context.Context) ([]*UserDefinedFunction, error) {
	return d.ExternalFunctionsFiltered(ctx, ObjectFilter{})
}

// ExternalFunctionsFiltered returns the external functions an ObjectFilter
// matches, narrowed by the server. An empty filter is ExternalFunctions.
func (d *Database) ExternalFunctionsFiltered(ctx context.Context, filter ObjectFilter) ([]*UserDefinedFunction, error) {
	where, args := filter.clause(allObjectsFilterColumns, 1)
	return d.functionsWhere(ctx, "o.type = 'XF'", where, args)
}

func (d *Database) functionsWhere(ctx context.Context, kind, where string, args []any) ([]*UserDefinedFunction, error) {
	q := `
SELECT o.object_id, SCHEMA_NAME(o.schema_id), o.name, o.type,
       o.create_date, o.modify_date
FROM   sys.objects o
WHERE  ` + kind + ` ` + where + `
ORDER  BY SCHEMA_NAME(o.schema_id), o.name`

	rows, err := d.query(ctx, q, args...)
	return scanRows(rows, err, fmt.Sprintf("list UDFs in %q", d.Name), func(scan func(...any) error) (*UserDefinedFunction, error) {
		f := &UserDefinedFunction{db: d}
		if err := scan(&f.ObjectID, &f.Schema, &f.Name, &f.FuncType,
			&f.CreateDate, &f.ModifyDate); err != nil {
			return nil, err
		}
		f.FuncType = FunctionType(strings.TrimSpace(string(f.FuncType)))
		return f, nil
	})
}

// SystemFunctions returns every system function SQL Server ships in the
// "sys" schema (sys.fn_listextendedproperty, ...) — see
// SystemFunctions.
//
// Reads sys.all_objects rather than sys.objects for the same reason
// SystemViews reads sys.all_objects instead of sys.views: shipped
// objects are invisible through the non-"all_" catalog views. Restricted to
// the three T-SQL types ('FN'/'TF'/'IF'): the shipped functions are T-SQL,
// and aggregates ('AF') are not functions DROP FUNCTION removes. The "sys"
// schema is identical in every database on a server, so this only needs
// loading once per connection.
func (d *Database) SystemFunctions(ctx context.Context) ([]*UserDefinedFunction, error) {
	return d.systemFunctionsWhere(ctx, "", nil)
}

// SystemFunctionsFiltered returns the system functions an ObjectFilter
// matches, narrowed by the server. An empty filter is SystemFunctions.
func (d *Database) SystemFunctionsFiltered(ctx context.Context, filter ObjectFilter) ([]*UserDefinedFunction, error) {
	where, args := filter.clause(allObjectsFilterColumns, 1)
	return d.systemFunctionsWhere(ctx, where, args)
}

func (d *Database) systemFunctionsWhere(ctx context.Context, where string, args []any) ([]*UserDefinedFunction, error) {
	q := `
SELECT o.object_id, SCHEMA_NAME(o.schema_id), o.name, o.type,
       o.create_date, o.modify_date
FROM   sys.all_objects o
WHERE  o.type IN ('FN','TF','IF') AND o.is_ms_shipped = 1 AND SCHEMA_NAME(o.schema_id) = 'sys' ` + where + `
ORDER  BY o.name`

	rows, err := d.query(ctx, q, args...)
	return scanRows(rows, err, fmt.Sprintf("list system UDFs in %q", d.Name), func(scan func(...any) error) (*UserDefinedFunction, error) {
		f := &UserDefinedFunction{db: d}
		if err := scan(&f.ObjectID, &f.Schema, &f.Name, &f.FuncType,
			&f.CreateDate, &f.ModifyDate); err != nil {
			return nil, err
		}
		f.FuncType = FunctionType(strings.TrimSpace(string(f.FuncType)))
		return f, nil
	})
}

// UserDefinedFunctionRef returns a lightweight handle for the function [schema].[name] — no
// query; every field but Schema and Name is zero. See Server.DatabaseRef for
// when a handle is the right form.
func (d *Database) UserDefinedFunctionRef(schema, name string) *UserDefinedFunction {
	return &UserDefinedFunction{db: d, Schema: schema, Name: name}
}

// Database returns the database the function belongs to.
func (f *UserDefinedFunction) Database() *Database { return f.db }

// Definition returns the function's T-SQL text: "" for an encrypted or CLR
// function, ErrNotFound when there is no such function. Listings do not
// carry it; this reads it by name, so a UserDefinedFunctionRef is enough.
func (f *UserDefinedFunction) Definition(ctx context.Context) (string, error) {
	return f.db.moduleDefinition(ctx, "function", "o.type IN ('FN','TF','IF','FS','FT','XF')", f.Schema, f.Name)
}

// Drop drops the function — scalar, inline table-valued or multi-statement
// table-valued, T-SQL or CLR alike, all of which DROP FUNCTION removes. A function that
// isn't there is the server's error, not a silent success — see the note on
// Table.Drop.
func (f *UserDefinedFunction) Drop(ctx context.Context) error {
	return f.db.dropSchemaObject(ctx, "function", "FUNCTION", f.Schema, f.Name)
}

// Rename renames the function (sp_rename's 'OBJECT' class). newName is a bare
// name; a rename never moves the function between schemas — see Transfer.
func (f *UserDefinedFunction) Rename(ctx context.Context, newName string) error {
	if err := f.db.renameSchemaObject(ctx, "function", renameObjectClass, f.Schema, f.Name, newName); err != nil {
		return err
	}
	setIfApplied(ctx, &f.Name, newName)
	return nil
}

// Transfer moves the function into another schema (ALTER SCHEMA ... TRANSFER).
// It keeps its name and object_id; permissions granted on it directly are
// dropped by the server.
func (f *UserDefinedFunction) Transfer(ctx context.Context, targetSchema string) error {
	if err := f.db.transferSchemaObject(ctx, "function", transferObjectClass, targetSchema, f.Schema, f.Name); err != nil {
		return err
	}
	setIfApplied(ctx, &f.Schema, targetSchema)
	return nil
}
