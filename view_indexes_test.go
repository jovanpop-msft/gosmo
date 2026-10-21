package gosmo

import (
	"context"
	"strings"
	"testing"
)

// A view's index and statistic answer View, not Table, however they were
// reached — by handle or as a create's result — and their statements name
// the view as it is now: the carrier is rebuilt per call, so a Rename of the
// view is followed rather than leaving the index naming the old one.
func TestViewIndexesAndStatisticsBelongToTheView(t *testing.T) {
	d := &Database{server: &Server{}, Name: "AppDB"}
	v := d.ViewRef("dbo", "vOrders")
	ctx, script := WithScript(context.Background())

	idx, err := v.CreateIndex(ctx, CreateIndexRequest{Name: "CIX_v", Type: IndexTypeClustered,
		IsUnique: true, KeyColumns: []IndexColumnDef{{Name: "ID"}}})
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	st, err := v.CreateStatistic(ctx, CreateStatisticRequest{Name: "st_v", Columns: []string{"n"}})
	if err != nil {
		t.Fatalf("CreateStatistic: %v", err)
	}
	for name, got := range map[string]struct {
		view  *View
		table bool
	}{
		"created index":     {idx.View(), idx.Table() != nil},
		"created statistic": {st.View(), st.Table() != nil},
		"index handle":      {v.IndexRef("ix").View(), v.IndexRef("ix").Table() != nil},
		"statistic handle":  {v.StatisticRef("st").View(), v.StatisticRef("st").Table() != nil},
	} {
		if got.view != v || got.table {
			t.Errorf("%s: View() = %p (want %p), Table() set = %v (want nil)", name, got.view, v, got.table)
		}
	}

	if err := v.Rename(ctx, "vOrders2"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	// Under WithScript the rename is only collected, so setIfApplied leaves
	// the name; set it as an applied rename would.
	v.Name = "vOrders2"
	if err := idx.Drop(ctx); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	stmts := script.Statements()
	want := []string{
		"CREATE UNIQUE CLUSTERED INDEX [CIX_v] ON [dbo].[vOrders] ([ID] ASC)",
		"CREATE STATISTICS [st_v] ON [dbo].[vOrders] ([n])",
		"",
		"DROP INDEX [CIX_v] ON [dbo].[vOrders2]",
	}
	if len(stmts) != len(want) {
		t.Fatalf("statements = %q, want %d", stmts, len(want))
	}
	for i, w := range want {
		if w != "" && !strings.HasSuffix(stmts[i], "\n"+w) {
			t.Errorf("statement %d = %q, want it to end in %q", i, stmts[i], w)
		}
	}
}
