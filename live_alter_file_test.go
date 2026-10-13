//go:build livedb

// Live verification that MODIFY FILE takes a size of 2 TB and up: the
// server parses the number as an int, so 2147483648KB is Msg 102 and the
// statement has to switch to MB.
//
//	go test -tags livedb . -run TestLiveAlterFile -v \
//	  -livedb 'sqlserver://sa:PASS@host?TrustServerCertificate=true'
//
// Creates and drops its own throwaway database; touches nothing else.
package gosmo

import "testing"

func TestLiveAlterFileMaxSizeOfTwoTerabytes(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	const name = "gosmo_alterfile_2tb"
	d, drop := liveScratchDB(t, db, ctx, name)
	defer drop()

	const twoTB = int64(2) << 30 // KB
	if err := d.FileRef(name).Alter(ctx, FileModify{MaxSizeKB: twoTB}); err != nil {
		t.Fatalf("Alter MaxSizeKB 2 TB: %v", err)
	}
	var pages int64
	if err := db.QueryRowContext(ctx,
		`SELECT max_size FROM sys.master_files WHERE database_id = DB_ID(@p1) AND name = @p1`, name).Scan(&pages); err != nil {
		t.Fatalf("read max_size: %v", err)
	}
	if want := twoTB / 8; pages != want {
		t.Errorf("max_size = %d pages, want %d", pages, want)
	}
}

// M5 (review plan 2026-10-13): a file grown to 64 MB shrinks to 16 MB through
// DBCC SHRINKFILE — MODIFY FILE with the smaller size is Msg 5039 — and the
// size Shrink reports is the one the catalog then shows. The data file is
// named by its logical name; a FileRef has no file_id.
func TestLiveShrinkFile(t *testing.T) {
	db, ctx, done := liveDB(t)
	defer done()
	const name = "gosmo_shrinkfile"
	d, drop := liveScratchDB(t, db, ctx, name)
	defer drop()

	f := d.FileRef(name)
	if err := f.Alter(ctx, FileModify{SizeKB: 64 * 1024}); err != nil {
		t.Fatalf("grow to 64 MB: %v", err)
	}
	if err := f.Alter(ctx, FileModify{SizeKB: 16 * 1024}); err == nil {
		t.Fatal("MODIFY FILE to a smaller size succeeded; Shrink would be unneeded")
	}
	got, err := f.Shrink(ctx, 16*1024)
	if err != nil {
		t.Fatalf("Shrink: %v", err)
	}
	var pages int64
	if err := db.QueryRowContext(ctx,
		`SELECT size FROM sys.master_files WHERE database_id = DB_ID(@p1) AND name = @p1`, name).Scan(&pages); err != nil {
		t.Fatalf("read size: %v", err)
	}
	if pages*8 != 16*1024 || got != pages*8 {
		t.Errorf("size = %d KB, Shrink reported %d KB, want both 16384", pages*8, got)
	}
	// Already at the target, and below what the data allows: still a row.
	for _, target := range []int64{16 * 1024, 0} {
		if got, err := f.Shrink(ctx, target); err != nil || got <= 0 || got > 16*1024 {
			t.Errorf("Shrink(%d) again = %d, %v", target, got, err)
		}
	}
	if got, err := d.FileRef(name+"_log").Shrink(ctx, 0); err != nil || got <= 0 {
		t.Errorf("Shrink of the log = %d, %v", got, err)
	}
}
