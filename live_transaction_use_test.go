//go:build livedb

// Live InTransaction: a parameterised database-scoped read, then a write in
// the same database. go-mssqldb sends a read with arguments as a TDS RPC call
// of sp_executesql. Its documented "a USE inside lasts only until it returns"
// holds for EXEC sp_executesql from T-SQL, not for a client's RPC: there the
// USE persists, so the read leaves the session in the database and the write,
// skipping its own USE, still lands there (checked on 13, 14 and 17,
// 2026-10-15). queryOn and queryRow rely on that when they record the
// database after a read; this pins it.
//
//	go test -tags livedb . -run TestLiveInTransactionParameterisedReadKeepsDatabase -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// The transaction is rolled back, so only the throwaway database is created.
package gosmo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

const liveTxUseDB = "gosmo_live_txuse_db"

func TestLiveInTransactionParameterisedReadKeepsDatabase(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)

	srv, err := NewServer(ctx, db)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	if _, err := db.ExecContext(ctx, "CREATE DATABASE ["+liveTxUseDB+"]"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DROP DATABASE IF EXISTS ["+liveTxUseDB+"]"); err != nil {
			t.Errorf("drop database: %v", err)
		}
	})
	d := srv.DatabaseRef(liveTxUseDB)

	reads := []struct {
		name string
		read func(ctx context.Context) error
	}{
		{"query", func(ctx context.Context) error {
			rows, err := d.query(ctx, "SELECT name FROM sys.objects WHERE name = @p1", "nothing")
			if err != nil {
				return err
			}
			return rows.Close()
		}},
		{"queryRow", func(ctx context.Context) error {
			var n int
			return d.queryRow(ctx, func(row *sql.Row) error { return row.Scan(&n) },
				"SELECT COUNT(*) FROM sys.objects WHERE name = @p1", "nothing")
		}},
	}
	errRollback := errors.New("roll back")
	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			var inDB, inMaster sql.NullInt64
			err := srv.InTransaction(ctx, func(ctx context.Context) error {
				if err := r.read(ctx); err != nil {
					return err
				}
				if _, err := d.exec(ctx, "CREATE TABLE dbo.gosmo_live_txuse (id int)"); err != nil {
					return err
				}
				if err := srv.queryRow(ctx, func(row *sql.Row) error { return row.Scan(&inDB, &inMaster) },
					"SELECT OBJECT_ID(N'["+liveTxUseDB+"].dbo.gosmo_live_txuse'), OBJECT_ID(N'master.dbo.gosmo_live_txuse')"); err != nil {
					return err
				}
				return errRollback
			})
			if !errors.Is(err, errRollback) {
				t.Fatalf("InTransaction: %v", err)
			}
			if !inDB.Valid || inMaster.Valid {
				t.Errorf("table in %s: %v, in master: %v — want it in %s only", liveTxUseDB, inDB.Valid, inMaster.Valid, liveTxUseDB)
			}
		})
	}
}
