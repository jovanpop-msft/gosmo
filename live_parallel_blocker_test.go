//go:build livedb

// Live verification that a request blocked inside a parallel plan is reported
// as blocked (gossms review plan 2026-10-16, U1). The lock wait of such a
// request sits on a worker task: dm_exec_requests shows the coordinator
// waiting on CXCONSUMER with blocking_session_id 0, and sysprocesses puts the
// blocker on an ecid > 0 row. Only the server can show which view names it.
//
//	go test -tags livedb . -run TestLiveParallelBlockerIsReported -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates a global temporary table, which goes when its connection closes;
// touches nothing else.
package gosmo

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestLiveParallelBlockerIsReported(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)
	srv, err := NewServer(ctx, db)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	conn := func() (*sql.Conn, int) {
		t.Helper()
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("connection: %v", err)
		}
		t.Cleanup(func() { c.Close() })
		var spid int
		if err := c.QueryRowContext(ctx, "SELECT @@SPID").Scan(&spid); err != nil {
			t.Fatalf("@@SPID: %v", err)
		}
		return c, spid
	}

	// The head: a table big enough for a parallel scan, one row locked in an
	// open transaction.
	head, headID := conn()
	for _, q := range []string{
		`DROP TABLE IF EXISTS ##gosmo_live_parblk`,
		`CREATE TABLE ##gosmo_live_parblk (id INT NOT NULL PRIMARY KEY, pad CHAR(100) NOT NULL DEFAULT 'x')`,
		`INSERT ##gosmo_live_parblk (id)
		 SELECT TOP (300000) ROW_NUMBER() OVER (ORDER BY (SELECT NULL))
		 FROM sys.all_objects a CROSS JOIN sys.all_objects b`,
		`BEGIN TRANSACTION`,
		`UPDATE ##gosmo_live_parblk SET pad = 'y' WHERE id = 150000`,
	} {
		if _, err := head.ExecContext(ctx, q); err != nil {
			t.Fatalf("head: %s: %v", q, err)
		}
	}
	t.Cleanup(func() { head.ExecContext(context.Background(), "IF @@TRANCOUNT > 0 ROLLBACK") })

	for _, tc := range []struct {
		name, hint string
		parallel   bool
	}{
		{"parallel", "OPTION (MAXDOP 4, USE HINT('ENABLE_PARALLEL_PLAN_PREFERENCE'))", true},
		// The control: the request's own task holds the lock wait.
		{"serial", "OPTION (MAXDOP 1)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, readerID := conn()
			rctx, cancel := context.WithCancel(ctx)
			finished := make(chan error, 1)
			go func() {
				var n int
				// The predicate keeps the plan out of TRIVIAL optimization, which
				// is always serial whatever the hint.
				finished <- reader.QueryRowContext(rctx, "SELECT COUNT(*) FROM ##gosmo_live_parblk WHERE pad <> 'z' "+tc.hint).Scan(&n)
			}()
			t.Cleanup(func() { cancel(); <-finished })

			// Wait for the lock wait to show on some task of the reader — on a
			// worker, for the parallel run.
			var tasks int
			deadline := time.Now().Add(15 * time.Second)
			for {
				var blocked int
				if err := db.QueryRowContext(ctx, `
SELECT (SELECT COUNT(*) FROM sys.dm_os_tasks WHERE session_id = @p1),
       (SELECT COUNT(*) FROM sys.dm_os_waiting_tasks WHERE session_id = @p1 AND blocking_session_id = @p2)`, readerID, headID).Scan(&tasks, &blocked); err != nil {
					t.Fatalf("waiting tasks: %v", err)
				}
				if blocked > 0 && (tasks > 1 || !tc.parallel) {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("reader %d (%d task(s)) never blocked on head %d; a parallel run needs a parallel plan", readerID, tasks, headID)
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !tc.parallel && tasks != 1 {
				t.Fatalf("serial reader %d runs %d tasks", readerID, tasks)
			}

			br, err := srv.BlockedRequests(ctx, os.Getpid())
			if err != nil {
				t.Fatalf("BlockedRequests: %v", err)
			}
			found := false
			for _, b := range br {
				if b.SessionID == readerID {
					found = true
					if b.BlockingSessionID != headID {
						t.Errorf("BlockedRequests: %d blocked by %d, want %d", readerID, b.BlockingSessionID, headID)
					}
					if len(b.WaitType) < 5 || b.WaitType[:5] != "LCK_M" {
						t.Errorf("BlockedRequests: wait type %q, want the worker's lock wait", b.WaitType)
					}
				}
			}
			if !found {
				t.Errorf("BlockedRequests(%d) = %+v, lacks reader %d", os.Getpid(), br, readerID)
			}

			act, err := srv.RequestActivity(ctx)
			if err != nil {
				t.Fatalf("RequestActivity: %v", err)
			}
			if act.BlockedRequests < 1 {
				t.Errorf("RequestActivity.BlockedRequests = %d, want the reader counted", act.BlockedRequests)
			}

			sessions, err := srv.ActiveSessions(ctx, UserSessionsOnly)
			if err != nil {
				t.Fatalf("ActiveSessions: %v", err)
			}
			found = false
			for _, s := range sessions {
				if s.SessionID == readerID {
					found = true
					if s.BlockingSessionID != headID {
						t.Errorf("ActiveSessions: %d blocked by %d, want %d", readerID, s.BlockingSessionID, headID)
					}
				}
			}
			if !found {
				t.Errorf("ActiveSessions lacks reader %d", readerID)
			}
		})
	}
}
