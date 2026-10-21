//go:build livedb

// Live verification that AlterColumn keeps a column's SPARSE format and its
// dynamic data mask (review plan 2026-10-21, B4): ALTER COLUMN resets both
// without a word when the statement does not restate them, so widening a
// masked column would expose its clear values. Asserted on sys.columns and
// sys.masked_columns read back.
//
//	go test -tags livedb . -run TestLiveAlterColumnKeepsSparseAndMask -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops one throwaway database; touches nothing else.
package gosmo

import (
	"database/sql"
	"errors"
	"testing"
)

func TestLiveAlterColumnKeepsSparseAndMask(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)

	d, drop := liveScratchDB(t, db, ctx, "gosmo_alter_attrs")
	t.Cleanup(drop)

	// CreateTable declares neither SPARSE nor MASKED, so the fixture is DDL.
	if _, err := d.exec(ctx, `CREATE TABLE dbo.Attrs (
    id int NOT NULL PRIMARY KEY,
    sp varchar(10) SPARSE NULL,
    mk varchar(10) MASKED WITH (FUNCTION = 'email()') NULL,
    both nvarchar(10) SPARSE MASKED WITH (FUNCTION = 'partial(1,"X''X",0)') NULL,
    plain int NULL)`); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	type attrs struct {
		sparse, masked bool
		fn             string
		typ            string
	}
	read := func(col string) attrs {
		t.Helper()
		var a attrs
		var fn sql.NullString
		if err := d.queryRow(ctx, func(row *sql.Row) error { return row.Scan(&a.sparse, &a.masked, &fn, &a.typ) }, `
SELECT c.is_sparse, c.is_masked, mc.masking_function,
       TYPE_NAME(c.user_type_id) + '(' + CONVERT(varchar(10), c.max_length) + ')'
FROM   sys.columns c
LEFT   JOIN sys.masked_columns mc ON mc.object_id = c.object_id AND mc.column_id = c.column_id
WHERE  c.object_id = OBJECT_ID(N'dbo.Attrs') AND c.name = @p1`, col); err != nil {
			t.Fatalf("read %s: %v", col, err)
		}
		a.fn = fn.String
		return a
	}
	want := map[string]attrs{
		"sp": {sparse: true, typ: "varchar(20)"},
		"mk": {masked: true, fn: "email()", typ: "varchar(20)"},
		// The catalog's own spelling of the function, read before any change,
		// so the comparison does not depend on how the server normalizes it.
		"both": {sparse: true, masked: true, fn: read("both").fn, typ: "nvarchar(60)"},
	}

	ref := d.TableRef("dbo", "Attrs")
	for _, col := range []ColumnDefinition{
		{Name: "sp", DataType: DataTypeVarChar, MaxLength: 20, IsNullable: true},
		{Name: "mk", DataType: DataTypeVarChar, MaxLength: 20, IsNullable: true},
		{Name: "both", DataType: DataTypeNVarChar, MaxLength: 30, IsNullable: true},
	} {
		// SQL Server 2016 cannot keep both: ADD MASKED clears SPARSE there.
		if col.Name == "both" && d.serverMajorVersion() == int(SQLServer2016) {
			before := read("both")
			if err := ref.AlterColumn(ctx, col); !errors.Is(err, ErrUnsupported) {
				t.Errorf("AlterColumn both on 2016 = %v, want ErrUnsupported", err)
			}
			if got := read("both"); got != before {
				t.Errorf("after a refused AlterColumn both: %+v, want it unchanged %+v", got, before)
			}
			continue
		}
		if err := ref.AlterColumn(ctx, col); err != nil {
			t.Fatalf("AlterColumn %s: %v", col.Name, err)
		}
		if got := read(col.Name); got != want[col.Name] {
			t.Errorf("after AlterColumn %s: %+v, want %+v", col.Name, got, want[col.Name])
		}
	}

	// A sparse column asked to be NOT NULL is refused before anything runs.
	if err := ref.AlterColumn(ctx, ColumnDefinition{Name: "sp", DataType: DataTypeVarChar, MaxLength: 20}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("AlterColumn sparse NOT NULL = %v, want ErrInvalidRequest", err)
	}
	// A mask the new type cannot take fails the whole change: the type stays.
	if err := ref.AlterColumn(ctx, ColumnDefinition{Name: "mk", DataType: DataTypeInt, IsNullable: true}); err == nil {
		t.Logf("AlterColumn mk to int with email() accepted; read back %+v", read("mk"))
	} else if got := read("mk"); got != want["mk"] {
		t.Errorf("after a failed AlterColumn mk: %+v, want it unchanged %+v (%v)", got, want["mk"], err)
	}
	// A plain column gains neither.
	if err := ref.AlterColumn(ctx, ColumnDefinition{Name: "plain", DataType: DataTypeBigInt, IsNullable: true}); err != nil {
		t.Fatalf("AlterColumn plain: %v", err)
	}
	if got := read("plain"); got.sparse || got.masked {
		t.Errorf("plain after AlterColumn: %+v, want neither sparse nor masked", got)
	}
}
