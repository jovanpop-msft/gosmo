//go:build livedb

// Live verification of M8 (review plan 2026-10-13): an index read as
// columnstore, or a memory-optimized table's, refuses the ALTER INDEX options
// the server refuses for its kind before sending them — and what the server
// does accept for it still runs.
//
//	go test -tags livedb . -run TestLiveIndexKindRefusals -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops its own throwaway database; touches nothing else.
package gosmo

import (
	"errors"
	"testing"
)

func TestLiveIndexKindRefusals(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	d, drop := liveScratchDB(t, db, ctx, "gosmo_index_kind_refusals")
	defer drop()

	if _, err := d.exec(ctx, "CREATE TABLE dbo.c (a int, b int); CREATE CLUSTERED COLUMNSTORE INDEX cci ON dbo.c"); err != nil {
		t.Fatalf("create columnstore table: %v", err)
	}
	tbl, err := d.TableByName(ctx, "dbo", "c")
	if err != nil {
		t.Fatalf("TableByName: %v", err)
	}
	cci, err := tbl.IndexByName(ctx, "cci")
	if err != nil {
		t.Fatalf("IndexByName: %v", err)
	}
	if !cci.Type.IsColumnStore() {
		t.Fatalf("cci read as %q", cci.Type)
	}
	for name, err := range map[string]error{
		"SetOptions":          cci.SetOptions(ctx, IndexSetOptions{AllowRowLocks: new(true)}),
		"Rebuild fill factor": cci.Rebuild(ctx, IndexRebuildOptions{FillFactor: 80}),
		"Rebuild ROW":         cci.Rebuild(ctx, IndexRebuildOptions{DataCompression: DataCompressionRow}),
	} {
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s on a clustered columnstore index: err = %v, want ErrInvalidRequest", name, err)
		}
	}
	if err := cci.Rebuild(ctx, IndexRebuildOptions{DataCompression: DataCompressionColumnstoreArchive}); err != nil {
		t.Errorf("Rebuild COLUMNSTORE_ARCHIVE: %v", err)
	}
	if err := cci.Rebuild(ctx, IndexRebuildOptions{}); err != nil {
		t.Errorf("plain Rebuild: %v", err)
	}
}
