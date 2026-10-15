package gosmo

import (
	"slices"
	"strings"
	"testing"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/radix29/gosmo/tsql"
)

func TestQuoteNameIfNeeded(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Orders", "Orders"},
		{"_x1", "_x1"},
		{"a$b#c@d", "a$b#c@d"},
		{"User", "[User]"}, // bare, it is the USER function
		{"desc", "[desc]"},
		{"current_timestamp", "[current_timestamp]"},
		{"Name", "Name"}, // not reserved
		{"", "[]"},
		{"1st", "[1st]"},
		{"@v", "[@v]"},
		{"#t", "[#t]"},
		{"$x", "[$x]"},
		{"order id", "[order id]"},
		{"a]b", "[a]]b]"},
		{"Größe", "[Größe]"}, // non-ASCII is always bracketed
	} {
		if got := QuoteNameIfNeeded(tc.in); got != tc.want {
			t.Errorf("QuoteNameIfNeeded(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsReservedKeyword(t *testing.T) {
	for _, w := range []string{"select", "USER", "Desc", "Asc", "file", "PLAN", "percent", "open",
		"current", "schema", "database", "option", "try_convert", "semanticsimilaritydetailstable"} {
		if !IsReservedKeyword(w) {
			t.Errorf("IsReservedKeyword(%q) = false", w)
		}
	}
	// ODBC and future keywords, and words that are keywords only in a
	// context: all accepted bare by the server (probed on 13 and 17).
	for _, w := range []string{"name", "cast", "apply", "ties", "offset", "date", "count", "within", "user1", ""} {
		if IsReservedKeyword(w) {
			t.Errorf("IsReservedKeyword(%q) = true", w)
		}
	}
	if n := len(reservedKeywords); n != 184 {
		t.Errorf("%d reserved keywords, want 184 (179 the server refuses + 5 documented ones it accepts)", n)
	}
}

func TestQuoteAnsiLiteral(t *testing.T) {
	if got := QuoteAnsiLiteral("it's"); got != "'it''s'" {
		t.Errorf("QuoteAnsiLiteral = %s", got)
	}
	if got := QuoteLiteral("it's"); got != "N'it''s'" {
		t.Errorf("QuoteLiteral = %s", got)
	}
	if got := UnquoteName("[a]]b]"); got != "a]b" {
		t.Errorf("UnquoteName = %s", got)
	}
}

// TestTSQLQuoterParity pins package tsql's quoting to the mssql driver's
// TSQLQuoter, which gosmo called directly before tsql existed: tsql writes
// the two rules out so that it need not import the driver.
func TestTSQLQuoterParity(t *testing.T) {
	q := mssql.TSQLQuoter{}
	for _, s := range []string{"", "a", "a]b", "]]", "[x]", "it's", "''", "é\x00\n", "a]'b\"c", "N'x'"} {
		if got, want := tsql.QuoteName(s), q.ID(s); got != want {
			t.Errorf("tsql.QuoteName(%q) = %s, driver %s", s, got, want)
		}
		if got, want := tsql.QuoteAnsiLiteral(s), q.Value(s); got != want {
			t.Errorf("tsql.QuoteAnsiLiteral(%q) = %s, driver %s", s, got, want)
		}
		if got, want := tsql.QuoteLiteral(s), "N"+q.Value(s); got != want {
			t.Errorf("tsql.QuoteLiteral(%q) = %s, driver N%s", s, got, want)
		}
	}
}

func TestSplitName(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"t", []string{"t"}},
		{"dbo.t", []string{"dbo", "t"}},
		{"[dbo].[t]", []string{"dbo", "t"}},
		{"[my.schema].[a.b]", []string{"my.schema", "a.b"}},
		{"[a]]b].[c]]]]d]", []string{"a]b", "c]]d"}},
		{`"x"."y"`, []string{"x", "y"}},
		{`"a""b".c`, []string{`a"b`, "c"}},
		{`["x"].c`, []string{`"x"`, "c"}},
		{`"[x]".c`, []string{"[x]", "c"}},
		{" dbo . t ", []string{"dbo", "t"}},
		{"db..t", []string{"db", "", "t"}},
		{"srv.db.dbo.t", []string{"srv", "db", "dbo", "t"}},
		{`DOMAIN\user`, []string{`DOMAIN\user`}},
		{"[ spaced ]", []string{" spaced "}},
	} {
		got, err := SplitName(tc.in)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("SplitName(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{
		"", " ", ".", "dbo.", ".t", "a.b.c.d.e", "[dbo", `"dbo`, "[a]b]", "dbo t",
		"a[b]", `a"b`, "a]b", "[a]x", "[a].[b] c", "[]",
	} {
		if got, err := SplitName(in); err == nil {
			t.Errorf("SplitName(%q) = %q, want an error", in, got)
		}
	}
	for _, parts := range [][]string{{"a]b", "c.d"}, {`"q"`, "[x]", " s "}, {"x"}} {
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = QuoteName(p)
		}
		if got, err := SplitName(strings.Join(quoted, ".")); err != nil || !slices.Equal(got, parts) {
			t.Errorf("SplitName of quoted %q = %q, %v", parts, got, err)
		}
	}
}
