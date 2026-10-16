//go:build livedb

// Live verification of the tempdb usage reads (review plan X1/X9 of
// 2026-10-15 in gossms).
//
//	go test -tags livedb . -run TestLiveTempDB -v -livedb '...'

package gosmo

import (
	"context"
	"testing"
)

// TestLiveTempDBObjectsCountsRowsOnce: a #temp table with a clustered and two
// nonclustered indexes adds its row count once, not once per index. The
// local-temp total is read before and after, so other sessions' #tables
// cancel out.
func TestLiveTempDBObjectsCountsRowsOnce(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)
	srv := &Server{db: db}

	localRows := func() int64 {
		t.Helper()
		objs, err := srv.TempDBObjects(ctx)
		if err != nil {
			t.Fatalf("TempDBObjects: %v", err)
		}
		for _, o := range objs {
			if o.Kind == TempDBLocalTemp {
				return o.Rows
			}
		}
		return 0
	}

	// The #table lives as long as this connection, so it is pinned.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	before := localRows()
	const n = 1000
	if _, err := conn.ExecContext(ctx, `
CREATE TABLE #gosmo_live_x1 (id int NOT NULL PRIMARY KEY CLUSTERED, a int, b int);
CREATE INDEX ix_a ON #gosmo_live_x1 (a);
CREATE INDEX ix_b ON #gosmo_live_x1 (b);
INSERT #gosmo_live_x1 (id, a, b)
SELECT TOP (1000) n, n, n
FROM (SELECT ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS n
      FROM sys.all_columns a CROSS JOIN sys.all_columns b) x;`); err != nil {
		t.Fatalf("create #table: %v", err)
	}
	t.Cleanup(func() { conn.ExecContext(context.Background(), "DROP TABLE #gosmo_live_x1") })

	if got := localRows() - before; got != n {
		t.Errorf("local temp rows grew by %d, want %d (rows counted once per index?)", got, n)
	}
}

// TestLiveTempDBFilesReadsLogUsage: the log file's used space is read like the
// data files', not left at zero — tempdb's log always holds something.
func TestLiveTempDBFilesReadsLogUsage(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)
	srv := &Server{db: db}

	files, err := srv.TempDBFiles(ctx)
	if err != nil {
		t.Fatalf("TempDBFiles: %v", err)
	}
	var logs, data int
	for _, f := range files {
		if f.UsedMB <= 0 || f.UsedMB > f.SizeMB {
			t.Errorf("%s (%s): used %.2f MB of %.2f MB", f.Name, f.Type, f.UsedMB, f.SizeMB)
		}
		if f.Type == "LOG" {
			logs++
		} else {
			data++
		}
	}
	if logs == 0 || data == 0 {
		t.Errorf("TempDBFiles = %+v, want data and log files", files)
	}
}
