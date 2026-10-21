//go:build livedb

// Live verification that a hypothetical index — what the Database Engine
// Tuning Advisor leaves behind, CREATE INDEX … WITH STATISTICS_ONLY = -1 — is
// read as one, left out of a table script instead of being scripted as a
// real CREATE INDEX, kept out of the fragmentation list, refused by the
// ALTER INDEX writes the server would silently ignore, and still droppable.
//
//	go test -tags livedb . -run TestLiveHypotheticalIndex -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops its own throwaway database; touches nothing else.
package gosmo

import (
	"errors"
	"strings"
	"testing"
)

func TestLiveHypotheticalIndex(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()

	d, drop := liveScratchDB(t, db, ctx, "gosmo_hypothetical_live")
	defer drop()

	liveExecIn(t, d, ctx,
		`CREATE TABLE dbo.T (ID INT NOT NULL CONSTRAINT PK_T PRIMARY KEY, C INT NULL)`,
		`CREATE INDEX IX_real ON dbo.T (C)`,
		`CREATE INDEX IX_hypo ON dbo.T (C) WITH STATISTICS_ONLY = -1`,
		`EXEC sp_addextendedproperty N'note', N'dta', N'SCHEMA', N'dbo', N'TABLE', N'T', N'INDEX', N'IX_hypo'`,
		// On a heap a hypothetical clustered index takes index_id 2, not 1.
		`CREATE TABLE dbo.H (ID INT NOT NULL)`,
		`CREATE CLUSTERED INDEX CX_hypo ON dbo.H (ID) WITH STATISTICS_ONLY = -1`,
	)

	tbl, err := d.TableByName(ctx, "dbo", "T")
	if err != nil {
		t.Fatalf("TableByName: %v", err)
	}
	idxs, err := tbl.Indexes(ctx)
	if err != nil {
		t.Fatalf("Indexes: %v", err)
	}
	hypo := map[string]bool{}
	for _, idx := range idxs {
		hypo[idx.Name] = idx.IsHypothetical
	}
	if want := map[string]bool{"PK_T": false, "IX_real": false, "IX_hypo": true}; len(hypo) != len(want) ||
		hypo["PK_T"] || hypo["IX_real"] || !hypo["IX_hypo"] {
		t.Fatalf("IsHypothetical by index = %v, want %v", hypo, want)
	}

	opts := DefaultScriptOptions()
	opts.ExtendedProperties = true
	sc := NewScripter(d, opts)
	for _, table := range []string{"T", "H"} {
		script, err := sc.ScriptTable(ctx, "dbo", table)
		if err != nil {
			t.Fatalf("ScriptTable %s: %v", table, err)
		}
		if strings.Contains(script, "_hypo] ON") || strings.Contains(script, "N'IX_hypo'") {
			t.Errorf("script of %s recreates the hypothetical index or its property:\n%s", table, script)
		}
		if !strings.Contains(script, "-- hypothetical index") {
			t.Errorf("script of %s has no comment for the skipped index:\n%s", table, script)
		}
		if table == "T" && !strings.Contains(script, "CREATE NONCLUSTERED INDEX [IX_real]") {
			t.Errorf("script of T lost the real index:\n%s", script)
		}
	}

	frag, err := tbl.FragmentationStats(ctx, FragmentationLimited)
	if err != nil {
		t.Fatalf("FragmentationStats: %v", err)
	}
	for _, f := range frag {
		if f.IndexName == "IX_hypo" {
			t.Error("FragmentationStats lists the hypothetical index")
		}
	}

	ix, err := tbl.IndexByName(ctx, "IX_hypo")
	if err != nil {
		t.Fatalf("IndexByName: %v", err)
	}
	if err := ix.Disable(ctx); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Disable on a hypothetical index: err = %v, want ErrInvalidRequest", err)
	}
	if err := ix.Drop(ctx); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	if _, err := tbl.IndexByName(ctx, "IX_hypo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("IndexByName after Drop: err = %v, want ErrNotFound", err)
	}
}
