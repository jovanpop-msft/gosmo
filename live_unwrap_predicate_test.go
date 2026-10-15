//go:build livedb

// Live verification that a scripted security policy whose predicate carries
// a ")" in a string literal and in a bracketed name replays: unwrapPredicate
// must strip the catalog's outer parentheses without counting those, or ADD
// FILTER PREDICATE fails with "Incorrect syntax near '('".
//
//	go test -tags livedb . -run TestLiveUnwrapPredicateSkipsQuotedParens -v \
//	-livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops its own throwaway database; touches nothing else.
package gosmo

import (
	"strings"
	"testing"
)

func TestLiveUnwrapPredicateSkipsQuotedParens(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)

	d, drop := liveScratchDB(t, db, ctx, "gosmo_unwrap_live")
	t.Cleanup(drop)

	liveExecIn(t, d, ctx,
		`CREATE TABLE dbo.t (id INT NOT NULL PRIMARY KEY, owner_name SYSNAME NOT NULL)`,
		`CREATE SCHEMA rls`,
		`CREATE FUNCTION rls.[fn)x](@owner AS SYSNAME, @tag AS NVARCHAR(10)) RETURNS TABLE WITH SCHEMABINDING
		 AS RETURN SELECT 1 AS ok WHERE @owner = USER_NAME() OR @tag = N')'`,
		`CREATE SECURITY POLICY rls.pol
		 ADD FILTER PREDICATE rls.[fn)x](owner_name, N')') ON dbo.t
		 WITH (STATE = OFF)`,
	)

	script, err := NewScripter(d, DefaultScriptOptions()).ScriptSecurityPolicy(ctx, "rls", "pol")
	if err != nil {
		t.Fatalf("ScriptSecurityPolicy: %v", err)
	}
	if !strings.Contains(script, "ADD FILTER PREDICATE [rls].[fn)x]([owner_name],N')') ON") {
		t.Errorf("predicate not unwrapped:\n%s", script)
	}

	liveExecIn(t, d, ctx, `DROP SECURITY POLICY rls.pol`)
	liveRunScript(t, d, ctx, script)
	if _, err := d.SecurityPolicyByName(ctx, "rls", "pol"); err != nil {
		t.Fatalf("replayed policy not found: %v", err)
	}
}
