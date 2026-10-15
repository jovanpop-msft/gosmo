//go:build livedb

// Live verification that Table.CheckWhereSyntax compiles a predicate without
// running it. Only a server can say whether sp_describe_first_result_set
// reports a bad predicate, and whether a statement smuggled into the
// predicate stayed unexecuted.
//
//	go test -tags livedb . -run TestLiveCheckWhereSyntax -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops its own throwaway database; touches nothing else.
package gosmo

import "testing"

func TestLiveCheckWhereSyntax(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()

	d, drop := liveScratchDB(t, db, ctx, "gosmo_checkwhere_live")
	defer drop()

	liveExecIn(t, d, ctx,
		`CREATE TABLE dbo.T (ID INT NOT NULL PRIMARY KEY, Amount INT NULL)`,
		`INSERT dbo.T (ID, Amount) VALUES (1, 10), (2, NULL)`)
	tbl := d.TableRef("dbo", "T")

	for _, tc := range []struct {
		name, predicate string
		valid           bool
	}{
		{"plain", "Amount > 0", true},
		{"server filter_definition form", "([Amount]>(0))", true},
		{"trailing comment", "Amount IS NULL -- note", true},
		{"unknown column", "Nope > 0", false},
		{"syntax error", "Amount >", false},
		{"second statement", "Amount > 0; CREATE TABLE dbo.Pwned1 (a INT)", false},
		{"delete after predicate", "1 = 1; DELETE FROM dbo.T", false},
		// Balanced to escape the parentheses: compiles, so it is reported
		// valid, but must still not run.
		{"escaped parentheses", "1 = 1); CREATE TABLE dbo.Pwned2 (a INT); SELECT 1 WHERE (1 = 1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tbl.CheckWhereSyntax(ctx, tc.predicate)
			if tc.valid && err != nil {
				t.Errorf("CheckWhereSyntax(%q) = %v, want valid", tc.predicate, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("CheckWhereSyntax(%q) = nil, want an error", tc.predicate)
			}
		})
	}

	if n := liveScalar(t, d, ctx, `SELECT CAST(COUNT(*) AS varchar(10)) FROM sys.tables WHERE name LIKE 'Pwned%'`); n != "0" {
		t.Errorf("%s Pwned table(s) created: CheckWhereSyntax executed its predicate", n)
	}
	if n := liveScalar(t, d, ctx, `SELECT CAST(COUNT(*) AS varchar(10)) FROM dbo.T`); n != "2" {
		t.Errorf("dbo.T has %s rows after the checks, want 2: CheckWhereSyntax executed its predicate", n)
	}
}
