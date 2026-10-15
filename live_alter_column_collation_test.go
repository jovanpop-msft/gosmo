//go:build livedb

// Live verification that AlterColumn keeps a string column's collation
// (review plan 2026-10-15, P2): ALTER COLUMN with no COLLATE resets a
// char/varchar/nchar/nvarchar column to the database default, so a length or
// nullability change on a column collated unlike its database must restate
// the collation. Asserted on sys.columns.collation_name read back.
//
//	go test -tags livedb . -run TestLiveAlterColumnKeepsCollation -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops one throwaway database; touches nothing else.
package gosmo

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestLiveAlterColumnKeepsCollation(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)

	d, drop := liveScratchDB(t, db, ctx, "gosmo_alter_collation")
	t.Cleanup(drop)

	var dbDefault string
	if err := d.queryRow(ctx, func(row *sql.Row) error { return row.Scan(&dbDefault) },
		"SELECT CONVERT(nvarchar(128), DATABASEPROPERTYEX(DB_NAME(), 'Collation'))"); err != nil {
		t.Fatalf("read database collation: %v", err)
	}
	// A column collation the database does not have, so a reset shows.
	kept := "Latin1_General_CS_AS"
	if strings.EqualFold(dbDefault, kept) {
		kept = "Latin1_General_BIN2"
	}

	tbl, err := d.CreateTable(ctx, CreateTableRequest{Schema: "dbo", Name: "Coll", Columns: []ColumnDefinition{
		{Name: "id", DataType: DataTypeInt, IsPrimaryKey: true},
		{Name: "s", DataType: DataTypeVarChar, MaxLength: 20, Collation: kept, IsNullable: true},
		{Name: "n", DataType: DataTypeNVarChar, MaxLength: 20, IsNullable: true},
		{Name: "i", DataType: DataTypeInt, IsNullable: true},
	}})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	collationOf := func(col string) string {
		t.Helper()
		var c sql.NullString
		if err := d.queryRow(ctx, func(row *sql.Row) error { return row.Scan(&c) },
			"SELECT collation_name FROM sys.columns WHERE object_id = OBJECT_ID(N'dbo.Coll') AND name = @p1", col); err != nil {
			t.Fatalf("read collation of %s: %v", col, err)
		}
		return c.String
	}
	if got := collationOf("s"); got != kept {
		t.Fatalf("CreateTable: s collated %q, want %q", got, kept)
	}
	if got := collationOf("n"); got != dbDefault {
		t.Fatalf("CreateTable: n collated %q, want the database default %q", got, dbDefault)
	}

	// Through a lookup-free handle too: the read resolves the table by name.
	ref := d.TableRef("dbo", "Coll")
	steps := []struct {
		name string
		col  ColumnDefinition
		want map[string]string
	}{
		{"nullability only keeps it", ColumnDefinition{Name: "s", DataType: DataTypeVarChar, MaxLength: 20},
			map[string]string{"s": kept}},
		{"length and type keep it", ColumnDefinition{Name: "s", DataType: DataTypeNVarChar, MaxLength: 40, IsNullable: true},
			map[string]string{"s": kept}},
		{"explicit collation changes it", ColumnDefinition{Name: "n", DataType: DataTypeNVarChar, MaxLength: 20, Collation: kept, IsNullable: true},
			map[string]string{"n": kept}},
		{"a string from int takes the database default", ColumnDefinition{Name: "i", DataType: DataTypeVarChar, MaxLength: 11, IsNullable: true},
			map[string]string{"i": dbDefault}},
	}
	for _, s := range steps {
		if err := ref.AlterColumn(ctx, s.col); err != nil {
			t.Fatalf("%s: AlterColumn: %v", s.name, err)
		}
		for col, want := range s.want {
			if got := collationOf(col); got != want {
				t.Errorf("%s: %s collated %q, want %q", s.name, col, got, want)
			}
		}
	}
	if err := tbl.AlterColumn(ctx, ColumnDefinition{Name: "missing", DataType: DataTypeVarChar, MaxLength: 5}); !errors.Is(err, ErrNotFound) {
		t.Errorf("AlterColumn of a missing column = %v, want ErrNotFound", err)
	}
}
