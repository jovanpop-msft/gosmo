//go:build livedb

// Live round trips of what a table or module script carries beyond the
// object itself: an indexed view's indexes, and the extended properties of
// the object, its columns, parameters, indexes and constraints, each value
// with its own base type.
//
//	go test -tags livedb . -run TestLiveScript.*ExtendedProperties -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops two throwaway databases; touches nothing else.
package gosmo

import (
	"slices"
	"strings"
	"testing"
)

// liveExtPropsOracle lists every extended property in the database with its
// value's base type, precision, scale, length and bytes, keyed by names
// rather than ids — an oracle written apart from objectExtendedProperties.
const liveExtPropsOracle = `
SELECT ep.class, ISNULL(OBJECT_SCHEMA_NAME(ep.major_id), '') AS sch, ISNULL(OBJECT_NAME(ep.major_id), '') AS obj,
       CASE ep.class WHEN 1 THEN ISNULL(COL_NAME(ep.major_id, ep.minor_id), '')
                     WHEN 2 THEN ISNULL((SELECT p.name FROM sys.parameters p WHERE p.object_id = ep.major_id AND p.parameter_id = ep.minor_id), '')
                     WHEN 7 THEN ISNULL((SELECT i.name FROM sys.indexes i WHERE i.object_id = ep.major_id AND i.index_id = ep.minor_id), '')
       END AS sub,
       ep.name,
       ISNULL(CONVERT(sysname, SQL_VARIANT_PROPERTY(ep.value, 'BaseType')), '') AS base,
       ISNULL(CONVERT(int, SQL_VARIANT_PROPERTY(ep.value, 'Precision')), -1) AS prec,
       ISNULL(CONVERT(int, SQL_VARIANT_PROPERTY(ep.value, 'Scale')), -1) AS scale,
       ISNULL(CONVERT(int, SQL_VARIANT_PROPERTY(ep.value, 'MaxLength')), -1) AS len,
       ISNULL(CONVERT(varchar(max), CONVERT(varbinary(max), ep.value), 1), 'NULL') AS bytes
FROM   sys.extended_properties ep
WHERE  ep.class IN (1, 2, 7)`

// liveViewIndexesOracle is every index on a view, with its key columns.
const liveViewIndexesOracle = `
SELECT OBJECT_NAME(i.object_id) AS v, i.name, i.type_desc, i.is_unique, i.is_disabled,
       STUFF((SELECT ',' + COL_NAME(ic.object_id, ic.column_id) FROM sys.index_columns ic
              WHERE ic.object_id = i.object_id AND ic.index_id = i.index_id
              ORDER BY ic.key_ordinal FOR XML PATH('')), 1, 1, '') AS cols
FROM   sys.indexes i JOIN sys.views v ON v.object_id = i.object_id
WHERE  i.type > 0`

func TestLiveScriptIndexedViewAndExtendedProperties(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	src, dropSrc := liveScratchDB(t, db, ctx, "gosmo_extprop_src")
	defer dropSrc()
	dst, dropDst := liveScratchDB(t, db, ctx, "gosmo_extprop_dst")
	defer dropDst()

	add := func(name, value, l1type, l1name, l2type, l2name string) string {
		l2 := ""
		if l2type != "" {
			l2 = ", @level2type = N'" + l2type + "', @level2name = N'" + l2name + "'"
		}
		return "DECLARE @v sql_variant = " + value + "; EXEC sys.sp_addextendedproperty @name = N'" + name +
			"', @value = @v, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'" + l1type +
			"', @level1name = N'" + l1name + "'" + l2
	}
	liveExecIn(t, src, ctx,
		`CREATE TABLE dbo.B (ID int NOT NULL CONSTRAINT PK_B PRIMARY KEY, Cat int NOT NULL,
		    Amt money NULL CONSTRAINT DF_B_Amt DEFAULT 0, CONSTRAINT CK_B_Cat CHECK (Cat > 0))`,
		`CREATE INDEX IX_B_Cat ON dbo.B (Cat)`,
		`CREATE VIEW dbo.vB WITH SCHEMABINDING AS SELECT Cat, COUNT_BIG(*) AS n, SUM(ISNULL(Amt, 0)) AS total FROM dbo.B GROUP BY Cat`,
		`CREATE UNIQUE CLUSTERED INDEX CIX_vB ON dbo.vB (Cat)`,
		`CREATE INDEX IX_vB_n ON dbo.vB (n)`,
		`CREATE PROCEDURE dbo.pB @cat int, @label nvarchar(20) = NULL AS SELECT * FROM dbo.B WHERE Cat = @cat`,
		`CREATE FUNCTION dbo.fB (@x int) RETURNS int AS BEGIN RETURN @x + 1 END`,
		`CREATE TRIGGER dbo.trB ON dbo.B AFTER INSERT AS SET NOCOUNT ON`,
		add("MS_Description", "N'The B table, it''s here'", "TABLE", "B", "", ""),
		add("MS_Description", "N'Category'", "TABLE", "B", "COLUMN", "Cat"),
		add("Version", "CAST(42 AS int)", "TABLE", "B", "", ""),
		add("Ratio", "CAST(0.1 AS float)", "TABLE", "B", "", ""),
		add("Small", "CAST(0.1 AS real)", "TABLE", "B", "", ""),
		add("Price", "CAST(12.3456 AS money)", "TABLE", "B", "COLUMN", "Amt"),
		add("Exact", "CAST(12.345 AS decimal(9,4))", "TABLE", "B", "COLUMN", "ID"),
		add("Stamp", "CAST('2026-10-13T12:34:56.997' AS datetime)", "TABLE", "B", "", ""),
		add("Stamp2", "CAST('2026-10-13 12:34:56.1234567' AS datetime2(7))", "TABLE", "B", "", ""),
		add("Offset", "CAST('2026-10-13 12:34:56.12 +02:00' AS datetimeoffset(2))", "TABLE", "B", "", ""),
		add("Day", "CAST('2026-10-13' AS date)", "TABLE", "B", "", ""),
		add("Clock", "CAST('12:34:56.1234567' AS time(7))", "TABLE", "B", "", ""),
		add("Short", "CAST('2026-10-13T12:34:00' AS smalldatetime)", "TABLE", "B", "", ""),
		add("Cents", "CAST(-3.5 AS smallmoney)", "TABLE", "B", "", ""),
		add("Num", "CAST(-7 AS numeric(5,0))", "TABLE", "B", "", ""),
		add("Char", "CAST('ab' AS char(4))", "TABLE", "B", "", ""),
		add("Bin", "CAST(0x01 AS binary(3))", "TABLE", "B", "", ""),
		add("Big", "CAST(-9223372036854775808 AS bigint)", "TABLE", "B", "", ""),
		add("Tiny", "CAST(255 AS tinyint)", "TABLE", "B", "", ""),
		add("Unicode", "N'Ünïcödé ✓ it''s'", "TABLE", "B", "", ""),
		add("Blob", "CAST(0x0A0BFF AS varbinary(10))", "TABLE", "B", "", ""),
		add("Ansi", "CAST('ascii' AS varchar(30))", "TABLE", "B", "", ""),
		add("Fixed", "CAST(N'fix' AS nchar(5))", "TABLE", "B", "", ""),
		add("Guid", "CAST('6F9619FF-8B86-D011-B42D-00C04FC964FF' AS uniqueidentifier)", "TABLE", "B", "", ""),
		add("Flag", "CAST(1 AS bit)", "TABLE", "B", "", ""),
		add("Empty", "NULL", "TABLE", "B", "", ""),
		add("MS_Description", "N'pk'", "TABLE", "B", "CONSTRAINT", "PK_B"),
		add("MS_Description", "N'default'", "TABLE", "B", "CONSTRAINT", "DF_B_Amt"),
		add("MS_Description", "N'check'", "TABLE", "B", "CONSTRAINT", "CK_B_Cat"),
		add("MS_Description", "N'index'", "TABLE", "B", "INDEX", "IX_B_Cat"),
		add("MS_Description", "N'view'", "VIEW", "vB", "", ""),
		add("MS_Description", "N'view column'", "VIEW", "vB", "COLUMN", "total"),
		add("MS_Description", "N'view clustered'", "VIEW", "vB", "INDEX", "CIX_vB"),
		add("MS_Description", "N'view nonclustered'", "VIEW", "vB", "INDEX", "IX_vB_n"),
		add("MS_Description", "N'proc'", "PROCEDURE", "pB", "", ""),
		add("MS_Description", "N'param'", "PROCEDURE", "pB", "PARAMETER", "@label"),
		add("MS_Description", "N'func'", "FUNCTION", "fB", "", ""),
		add("MS_Description", "N'func param'", "FUNCTION", "fB", "PARAMETER", "@x"),
		add("MS_Description", "N'trigger'", "TABLE", "B", "TRIGGER", "trB"),
	)

	sc := NewScripter(src, DefaultScriptOptions())
	scriptAll := func(sc *Scripter) []string {
		t.Helper()
		var out []string
		for _, f := range []func() (string, error){
			func() (string, error) { return sc.ScriptTable(ctx, "dbo", "B") },
			func() (string, error) { return sc.ScriptView(ctx, "dbo", "vB") },
			func() (string, error) { return sc.ScriptStoredProcedure(ctx, "dbo", "pB") },
			func() (string, error) { return sc.ScriptFunction(ctx, "dbo", "fB") },
			func() (string, error) { return sc.ScriptTrigger(ctx, "dbo", "trB") },
		} {
			s, err := f()
			if err != nil {
				t.Fatalf("script: %v", err)
			}
			out = append(out, s)
		}
		return out
	}
	compare := func(what string) {
		t.Helper()
		for _, q := range []string{liveExtPropsOracle, liveViewIndexesOracle} {
			want := liveRowsAsStrings(t, src, ctx, q)
			got := liveRowsAsStrings(t, dst, ctx, q)
			if len(want) == 0 {
				t.Fatalf("%s: the source has nothing to compare", what)
			}
			for _, w := range want {
				if !slices.Contains(got, w) {
					t.Errorf("%s: missing or different after the round trip:\n  %s", what, w)
				}
			}
			for _, g := range got {
				if !slices.Contains(want, g) {
					t.Errorf("%s: not in the source:\n  %s", what, g)
				}
			}
		}
	}

	creates := scriptAll(sc)
	for _, s := range creates {
		livePinnedRun(t, db, ctx, dst.Name, s)
	}
	compare("CREATE")

	// ALTER VIEW drops the view's indexes and with them their properties;
	// the script puts both back, and its guards skip what ALTER kept.
	alterOpts := DefaultScriptOptions()
	alterOpts.Verb = ScriptAlter
	alterOpts.IncludeIfNotExists = false
	for i, s := range scriptAll(NewScripter(src, alterOpts)) {
		if i == 0 {
			continue // a table has no ALTER form; its script is the CREATE
		}
		livePinnedRun(t, db, ctx, dst.Name, s)
	}
	compare("ALTER")

	// DROP AND CREATE over the copy, guards on. The view first, alone: its
	// indexes and their properties go with the DROP and come back. Then the
	// table, which the schema-bound view blocks until it is dropped, and
	// whose DROP takes the trigger along.
	dc := DefaultScriptOptions()
	dc.Verb = ScriptDropAndCreate
	scripts := scriptAll(NewScripter(src, dc))
	livePinnedRun(t, db, ctx, dst.Name, scripts[1])
	compare("DROP AND CREATE of the view")
	drop := DefaultScriptOptions()
	drop.Verb = ScriptDrop
	dropView, err := NewScripter(src, drop).ScriptView(ctx, "dbo", "vB")
	if err != nil {
		t.Fatal(err)
	}
	livePinnedRun(t, db, ctx, dst.Name, dropView)
	livePinnedRun(t, db, ctx, dst.Name, scripts[0])
	livePinnedRun(t, db, ctx, dst.Name, creates[4])
	livePinnedRun(t, db, ctx, dst.Name, creates[1])
	compare("DROP AND CREATE")

	// Off, nothing is restated.
	off := DefaultScriptOptions()
	off.ExtendedProperties = false
	for _, s := range scriptAll(NewScripter(src, off)) {
		if strings.Contains(s, "sp_addextendedproperty") {
			t.Errorf("ExtendedProperties off, but:\n%s", s)
		}
	}
}
