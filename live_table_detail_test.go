//go:build livedb

// Live verification of Table.Detail's owner and the table space breakdown
// (review plan X2/X3 of 2026-10-15 in gossms).
//
//	go test -tags livedb . -run 'TestLiveTableDetailOwner|TestLiveTableSpaceParts' -v -livedb '...'

package gosmo

import "testing"

// TestLiveTableDetailOwner: a table re-owned with ALTER AUTHORIZATION ON
// OBJECT:: reports that principal, not its schema's owner; a table that
// never was reports the schema owner and says so.
func TestLiveTableDetailOwner(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	d, drop := liveScratchDB(t, db, ctx, "gosmo_live_x2")
	defer drop()

	liveExecIn(t, d, ctx,
		"CREATE USER x2_owner WITHOUT LOGIN",
		"CREATE TABLE dbo.Owned (id int)",
		"CREATE TABLE dbo.Inherited (id int)",
		"ALTER AUTHORIZATION ON OBJECT::dbo.Owned TO x2_owner",
	)
	for _, tc := range []struct {
		table, owner string
		inherited    bool
	}{
		{"Owned", "x2_owner", false},
		{"Inherited", "dbo", true},
	} {
		tb, err := d.TableByName(ctx, "dbo", tc.table)
		if err != nil {
			t.Fatalf("TableByName %s: %v", tc.table, err)
		}
		det, err := tb.Detail(ctx)
		if err != nil {
			t.Fatalf("Detail %s: %v", tc.table, err)
		}
		if det.Owner != tc.owner || det.OwnedBySchemaOwner != tc.inherited {
			t.Errorf("%s: Owner %q, OwnedBySchemaOwner %v; want %q, %v",
				tc.table, det.Owner, det.OwnedBySchemaOwner, tc.owner, tc.inherited)
		}
	}
}

// TestLiveTableSpacePartsSumToReserved: a nonclustered index INCLUDEing a
// filled nvarchar(max) column has LOB_DATA pages of its own; they count in
// LOBKB only, so the four parts add up to ReservedKB — for SpaceUsed and for
// TableSpaceUsedAll alike.
func TestLiveTableSpacePartsSumToReserved(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	d, drop := liveScratchDB(t, db, ctx, "gosmo_live_x3")
	defer drop()

	liveExecIn(t, d, ctx,
		"CREATE TABLE dbo.Lobby (id int NOT NULL PRIMARY KEY, k int, doc nvarchar(max))",
		"CREATE INDEX ix_k ON dbo.Lobby (k) INCLUDE (doc)",
		`INSERT dbo.Lobby (id, k, doc)
SELECT TOP (200) n, n, REPLICATE(CAST(N'x' AS nvarchar(max)), 20000)
FROM (SELECT ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS n
      FROM sys.all_columns) x`,
	)
	tb, err := d.TableByName(ctx, "dbo", "Lobby")
	if err != nil {
		t.Fatalf("TableByName: %v", err)
	}
	one, err := tb.SpaceUsed(ctx)
	if err != nil {
		t.Fatalf("SpaceUsed: %v", err)
	}
	all, err := d.TableSpaceUsedAll(ctx)
	if err != nil {
		t.Fatalf("TableSpaceUsedAll: %v", err)
	}
	for name, s := range map[string]*TableSpaceInfo{"SpaceUsed": one, "TableSpaceUsedAll": all[tb.ObjectID]} {
		if s == nil {
			t.Fatalf("%s: no row for the table", name)
		}
		t.Logf("%s: reserved %d = data %d + index %d + lob %d + unused %d",
			name, s.ReservedKB, s.DataKB, s.IndexKB, s.LOBKB, s.UnusedKB)
		if sum := s.DataKB + s.IndexKB + s.LOBKB + s.UnusedKB; sum != s.ReservedKB {
			t.Errorf("%s: parts sum to %d KB, reserved is %d KB", name, sum, s.ReservedKB)
		}
		if s.IndexKB <= 0 || s.LOBKB <= 0 {
			t.Errorf("%s: want both index and LOB space, got index %d, lob %d", name, s.IndexKB, s.LOBKB)
		}
	}
}
