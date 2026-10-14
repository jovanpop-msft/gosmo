package gosmo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"
)

// TestScriptExtendedPropertyWrites pins the sp_addextendedproperty family.
// See script_write_common_test.go.
//
// These are the sharpest case for a whole-statement assertion in the package.
// Every one of the eight level arguments is a *literal*, not an identifier, so
// nothing brackets them and the only thing standing between a table named
// O'Brien and a syntax error is escapeSingle. The level pairs also carry a
// second rule: an unused level is the NULL keyword, not an empty string
// literal — nullableStr maps an empty string to NULL for exactly that, and a
// build that quoted it instead would look right and address the wrong object
// level.
func TestScriptExtendedPropertyWrites(t *testing.T) {
	tableLevel := ExtendedPropertyLevel{
		Level0Type: "SCHEMA", Level0Name: "dbo",
		Level1Type: "TABLE", Level1Name: "Sales'Archive",
	}
	columnLevel := ExtendedPropertyLevel{
		Level0Type: "SCHEMA", Level0Name: "dbo",
		Level1Type: "TABLE", Level1Name: "Sales'Archive",
		Level2Type: "COLUMN", Level2Name: "O'Brien",
	}

	runScriptCases(t, []scriptCase{
		{"AddExtendedProperty", func(c context.Context) error {
			return scriptTestDB().AddExtendedProperty(c, "MS_Description", "o'brien", tableLevel)
		}, scriptUsePrefix + `
EXEC sp_addextendedproperty
    @name = N'MS_Description', @value = N'o''brien',
    @level0type = N'SCHEMA', @level0name = N'dbo',
    @level1type = N'TABLE', @level1name = N'Sales''Archive',
    @level2type = NULL, @level2name = NULL`},
		{"AddExtendedProperty at column level", func(c context.Context) error {
			return scriptTestDB().AddExtendedProperty(c, "MS_Description", "v", columnLevel)
		}, scriptUsePrefix + `
EXEC sp_addextendedproperty
    @name = N'MS_Description', @value = N'v',
    @level0type = N'SCHEMA', @level0name = N'dbo',
    @level1type = N'TABLE', @level1name = N'Sales''Archive',
    @level2type = N'COLUMN', @level2name = N'O''Brien'`},
		{"SetExtendedProperty", func(c context.Context) error {
			return scriptTestDB().SetExtendedProperty(c, "MS_Description", "v'2", tableLevel)
		}, scriptUsePrefix + `
EXEC sp_updateextendedproperty
    @name = N'MS_Description', @value = N'v''2',
    @level0type = N'SCHEMA', @level0name = N'dbo',
    @level1type = N'TABLE', @level1name = N'Sales''Archive',
    @level2type = NULL, @level2name = NULL`},
		{"DropExtendedProperty", func(c context.Context) error {
			return scriptTestDB().DropExtendedProperty(c, "MS_Description", tableLevel)
		}, scriptUsePrefix + `
EXEC sp_dropextendedproperty
    @name = N'MS_Description',
    @level0type = N'SCHEMA', @level0name = N'dbo',
    @level1type = N'TABLE', @level1name = N'Sales''Archive',
    @level2type = NULL, @level2name = NULL`},
	})
}

// TestExtendedPropertyStatements pins the sp_addextendedproperty a script
// restates each property with (ScriptOptions.ExtendedProperties), one case
// per level a scripted object owns. The names carry an apostrophe and a
// bracket: every level argument is a literal, so only QuoteLiteral stands
// between them and a syntax error. A value keeps its base type: nvarchar as
// an N'…' literal, anything else converted back into a variable first.
func TestExtendedPropertyStatements(t *testing.T) {
	nv := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
	table := ExtendedPropertyLevel{Level0Type: "SCHEMA", Level0Name: "dbo", Level1Type: "TABLE", Level1Name: "o'brien"}
	at := func(l1, l1name, l2, l2name string) ExtendedPropertyLevel {
		return ExtendedPropertyLevel{Level0Type: "SCHEMA", Level0Name: "s]x", Level1Type: l1, Level1Name: l1name, Level2Type: l2, Level2Name: l2name}
	}
	const tail = ";\nGO\n"
	cases := []struct {
		name  string
		p     scriptedExtProp
		guard bool
		want  string
	}{
		{"table", scriptedExtProp{level: table, name: "MS_Description", baseType: "nvarchar", value: nv("it's")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = N'it''s', @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"column", scriptedExtProp{level: at("TABLE", "t", "COLUMN", "a]b"), name: "MS_Description", baseType: "nvarchar", value: nv("c")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = N'c', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'TABLE', @level1name = N't', @level2type = N'COLUMN', @level2name = N'a]b'" + tail},
		{"view index", scriptedExtProp{level: at("VIEW", "v", "INDEX", "CIX"), name: "n", baseType: "nvarchar", value: nv("i")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'n', @value = N'i', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'VIEW', @level1name = N'v', @level2type = N'INDEX', @level2name = N'CIX'" + tail},
		{"constraint", scriptedExtProp{level: at("TABLE", "t", "CONSTRAINT", "PK_t"), name: "n", baseType: "nvarchar", value: nv("k")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'n', @value = N'k', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'TABLE', @level1name = N't', @level2type = N'CONSTRAINT', @level2name = N'PK_t'" + tail},
		{"procedure parameter", scriptedExtProp{level: at("PROCEDURE", "p", "PARAMETER", "@x"), name: "n", baseType: "nvarchar", value: nv("p")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'n', @value = N'p', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'PROCEDURE', @level1name = N'p', @level2type = N'PARAMETER', @level2name = N'@x'" + tail},
		{"function", scriptedExtProp{level: at("FUNCTION", "f", "", ""), name: "n", baseType: "nvarchar", value: nv("f")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'n', @value = N'f', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'FUNCTION', @level1name = N'f'" + tail},
		{"trigger", scriptedExtProp{level: at("TABLE", "t", "TRIGGER", "tr"), name: "n", baseType: "nvarchar", value: nv("tr")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'n', @value = N'tr', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'TABLE', @level1name = N't', @level2type = N'TRIGGER', @level2name = N'tr'" + tail},
		{"int value", scriptedExtProp{level: table, name: "Version", baseType: "int", precision: 10, maxLength: 4, value: nv("42")}, false,
			"DECLARE @value sql_variant = CONVERT(int, N'42');\n" +
				"EXEC sys.sp_addextendedproperty @name = N'Version', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"decimal value", scriptedExtProp{level: table, name: "d", baseType: "decimal", precision: 9, scale: 3, value: nv("1.250")}, false,
			"DECLARE @value sql_variant = CONVERT(decimal(9,3), N'1.250');\n" +
				"EXEC sys.sp_addextendedproperty @name = N'd', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"varbinary value", scriptedExtProp{level: table, name: "b", baseType: "varbinary", maxLength: 2, value: nv("0x0AFF")}, false,
			"DECLARE @value sql_variant = CONVERT(varbinary(2), N'0x0AFF', 1);\n" +
				"EXEC sys.sp_addextendedproperty @name = N'b', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"nchar value", scriptedExtProp{level: table, name: "c", baseType: "nchar", maxLength: 6, value: nv("abc")}, false,
			"DECLARE @value sql_variant = CONVERT(nchar(3), N'abc');\n" +
				"EXEC sys.sp_addextendedproperty @name = N'c', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"datetime2 value", scriptedExtProp{level: table, name: "w", baseType: "datetime2", scale: 3, value: nv("2026-10-13 12:00:00.123")}, false,
			"DECLARE @value sql_variant = CONVERT(datetime2(3), N'2026-10-13 12:00:00.123');\n" +
				"EXEC sys.sp_addextendedproperty @name = N'w', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"NULL value", scriptedExtProp{level: table, name: "empty"}, false,
			"EXEC sys.sp_addextendedproperty @name = N'empty', @value = NULL, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"guarded", scriptedExtProp{level: at("TABLE", "t", "COLUMN", "c"), name: "it's", baseType: "nvarchar", value: nv("v")}, true,
			"IF NOT EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'it''s', N'SCHEMA', N's]x', N'TABLE', N't', N'COLUMN', N'c'))\n" +
				"    EXEC sys.sp_addextendedproperty @name = N'it''s', @value = N'v', @level0type = N'SCHEMA', @level0name = N's]x', @level1type = N'TABLE', @level1name = N't', @level2type = N'COLUMN', @level2name = N'c'" + tail},
		{"guarded object level, int value", scriptedExtProp{level: table, name: "n", baseType: "bigint", value: nv("-1")}, true,
			"DECLARE @value sql_variant = CONVERT(bigint, N'-1');\n" +
				"IF NOT EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'n', N'SCHEMA', N'dbo', N'TABLE', N'o''brien', NULL, NULL))\n" +
				"    EXEC sys.sp_addextendedproperty @name = N'n', @value = @value, @level0type = N'SCHEMA', @level0name = N'dbo', @level1type = N'TABLE', @level1name = N'o''brien'" + tail},
		{"unknown base type", scriptedExtProp{level: at("TABLE", "t", "COLUMN", "c"), name: "x", baseType: "hierarchyid", value: nv("/1/")}, false,
			"-- Extended property N'x' on [s]]x].[t].[c] holds a hierarchyid value with no literal form: not scripted.\n"},
		{"database", scriptedExtProp{name: "MS_Description", baseType: "nvarchar", value: nv("db")}, false,
			"EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = N'db'" + tail},
		{"guarded database", scriptedExtProp{name: "n", baseType: "int", value: nv("1")}, true,
			"DECLARE @value sql_variant = CONVERT(int, N'1');\n" +
				"IF NOT EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'n', NULL, NULL, NULL, NULL, NULL, NULL))\n" +
				"    EXEC sys.sp_addextendedproperty @name = N'n', @value = @value" + tail},
		{"schema", scriptedExtProp{level: ExtendedPropertyLevel{Level0Type: "SCHEMA", Level0Name: "s'x"}, name: "n", baseType: "nvarchar", value: nv("s")}, true,
			"IF NOT EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'n', N'SCHEMA', N's''x', NULL, NULL, NULL, NULL))\n" +
				"    EXEC sys.sp_addextendedproperty @name = N'n', @value = N's', @level0type = N'SCHEMA', @level0name = N's''x'" + tail},
		{"unknown base type on the database", scriptedExtProp{name: "x", baseType: "hierarchyid", value: nv("/1/")}, false,
			"-- Extended property N'x' on the database holds a hierarchyid value with no literal form: not scripted.\n"},
	}
	for _, c := range cases {
		if got := extendedPropertyStatement(c.p, c.guard); got != c.want {
			t.Errorf("%s:\n got: %q\nwant: %q", c.name, got, c.want)
		}
	}
}

// ScriptSchema reads its schema's own properties (class 3, by schema id) and
// restates them at SCHEMA level after the CREATE; a DROP alone reads none.
func TestScriptSchemaExtendedProperties(t *testing.T) {
	ctx := context.Background()
	d := captureDatabase(t)
	canned := []cannedRow{
		{match: "FROM   sys.schemas s", cols: []string{"name", "id", "owner"}, row: []driver.Value{"Sales", int64(7), "dbo"}},
		{match: "FROM   sys.extended_properties ep", cols: make([]string, 6),
			row: []driver.Value{"MS_Description", "nvarchar", int64(0), int64(0), int64(12), "orders"}},
	}
	captured.reset(canned...)
	got, err := NewScripter(d, DefaultScriptOptions()).ScriptSchema(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	want := "IF NOT EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'MS_Description', N'SCHEMA', N'Sales', NULL, NULL, NULL, NULL))\n" +
		"    EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = N'orders', @level0type = N'SCHEMA', @level0name = N'Sales';\nGO\n"
	if !strings.HasSuffix(got, want) {
		t.Errorf("schema property not restated:\n%s", got)
	}
	if q, args := captured.findArgs("FROM   sys.extended_properties ep"); !strings.Contains(q, "ep.class = @p1 AND ep.major_id = @p2 AND ep.minor_id = @p3") ||
		len(args) != 3 || args[0] != int64(3) || args[1] != int64(7) || args[2] != int64(0) {
		t.Errorf("read keyed wrong: %v\n%s", args, q)
	}

	drop := DefaultScriptOptions()
	drop.Verb = ScriptDrop
	captured.reset(canned...)
	if got, err := NewScripter(d, drop).ScriptSchema(ctx, "Sales"); err != nil || strings.Contains(got, "sp_addextendedproperty") ||
		captured.count("sys.extended_properties") != 0 {
		t.Errorf("DROP reads or restates properties (%v):\n%s", err, got)
	}
}
