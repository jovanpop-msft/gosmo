//go:build livedb

// Live verification of Server.ApplyConfiguration on a server where "show
// advanced options" is 0 — the installation default, and the state in which a
// bare sp_configure of an advanced option fails Msg 15123. win10cli\SQL2016
// and \SQL2017 ship that way; the 2025 default instance has it at 1, where
// the test still checks that the option is left alone.
//
//	go test -tags livedb . -run TestLiveApplyConfiguration -v \
//	  -livedb 'sqlserver://sa:PASS@host/sql2016?TrustServerCertificate=true&encrypt=false'
//
// Value-preserving: it sets max degree of parallelism and cost threshold for
// parallelism to their current values, and asserts "show advanced options"
// ends where it started.
package gosmo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	mssql "github.com/microsoft/go-mssqldb"
)

func TestLiveApplyConfiguration(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	srv, err := NewServer(ctx, db)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	read := func(name string) (value, inUse int64) {
		t.Helper()
		if err := db.QueryRowContext(ctx,
			`SELECT CONVERT(bigint, value), CONVERT(bigint, value_in_use) FROM sys.configurations WHERE name = @p1`,
			name).Scan(&value, &inUse); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return value, inUse
	}
	advBefore, advInUse := read(showAdvancedOptions)
	if advBefore != advInUse {
		t.Skipf("show advanced options has a pending change (%d, in use %d); not touching it", advBefore, advInUse)
	}
	maxdop, _ := read("max degree of parallelism")
	cost, _ := read("cost threshold for parallelism")

	if advBefore == 0 {
		// The trap ApplyConfiguration exists for: the raw primitive fails.
		err := srv.ConfigurationRef("max degree of parallelism").SetValue(ctx, maxdop)
		msErr, ok := errors.AsType[mssql.Error](err)
		if !ok || !slices.ContainsFunc(msErr.All, func(e mssql.Error) bool { return e.Number == 15123 }) {
			t.Errorf("bare SetValue of an advanced option with show advanced options = 0: %v, want Msg 15123", err)
		}
	}

	if err := srv.ApplyConfiguration(ctx, []ConfigChange{
		{Name: "max degree of parallelism", Value: maxdop},
		{Name: "cost threshold for parallelism", Value: cost},
	}, ConfigApplyOptions{}); err != nil {
		t.Fatalf("ApplyConfiguration: %v", err)
	}
	if v, u := read(showAdvancedOptions); v != advBefore || u != advBefore {
		t.Errorf("show advanced options ended at %d (in use %d), want %d", v, u, advBefore)
	}
	if v, u := read("max degree of parallelism"); v != maxdop || u != maxdop {
		t.Errorf("max degree of parallelism = %d (in use %d), want %d", v, u, maxdop)
	}

	// A failing change is reported, and the restore still runs.
	err = srv.ApplyConfiguration(ctx, []ConfigChange{
		{Name: "max degree of parallelism", Value: maxdop},
		{Name: "gosmo no such option", Value: 1},
	}, ConfigApplyOptions{})
	if err == nil {
		t.Error("ApplyConfiguration with an unknown option: nil error")
	}
	if v, u := read(showAdvancedOptions); v != advBefore || u != advBefore {
		t.Errorf("after a failed change, show advanced options ended at %d (in use %d), want %d", v, u, advBefore)
	}
}

// N7 (review plan 2026-10-13): "show advanced options" configured 1 but not
// yet installed (sp_configure without RECONFIGURE) has value_in_use 0, so the
// batch turns it on itself — and must then restore the configured 1, not 0.
//
// Sets the option's value and value_in_use to that state and restores both
// afterwards; the change it applies is max degree of parallelism to its
// current value.
func TestLiveApplyConfigurationKeepsAPendingShowAdvancedOptions(t *testing.T) {
	db, ctx, done := liveDB(t)
	t.Cleanup(done)
	srv, err := NewServer(ctx, db)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	read := func(name string) (value, inUse int64) {
		t.Helper()
		if err := db.QueryRowContext(ctx,
			`SELECT CONVERT(bigint, value), CONVERT(bigint, value_in_use) FROM sys.configurations WHERE name = @p1`,
			name).Scan(&value, &inUse); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return value, inUse
	}
	exec := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	advValue, advInUse := read(showAdvancedOptions)
	t.Cleanup(func() {
		c := context.Background()
		db.ExecContext(c, fmt.Sprintf("EXEC sys.sp_configure N'show advanced options', %d; RECONFIGURE;", advInUse))
		db.ExecContext(c, fmt.Sprintf("EXEC sys.sp_configure N'show advanced options', %d;", advValue))
		if v, u := read(showAdvancedOptions); v != advValue || u != advInUse {
			t.Errorf("restore: show advanced options is %d (in use %d), was %d (in use %d)", v, u, advValue, advInUse)
		}
	})
	exec("EXEC sys.sp_configure N'show advanced options', 0; RECONFIGURE;")
	exec("EXEC sys.sp_configure N'show advanced options', 1;")
	if v, u := read(showAdvancedOptions); v != 1 || u != 0 {
		t.Fatalf("setup: show advanced options is %d (in use %d), want 1 (in use 0)", v, u)
	}

	maxdop, _ := read("max degree of parallelism")
	if err := srv.ApplyConfiguration(ctx, []ConfigChange{{Name: "max degree of parallelism", Value: maxdop}},
		ConfigApplyOptions{}); err != nil {
		t.Fatalf("ApplyConfiguration: %v", err)
	}
	if v, _ := read(showAdvancedOptions); v != 1 {
		t.Errorf("show advanced options configured %d after ApplyConfiguration, want the pending 1", v)
	}
}
