package gosmo

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// TestScriptShrinkWrites pins DBCC SHRINKDATABASE and DBCC SHRINKFILE as
// Database.Shrink and DatabaseFileInfo.ShrinkWith render them. The zero
// options are the bare SHRINKDATABASE SSMS scripts with "Reorganize files"
// unticked (captured from SSMS, 2026-10-21); a file target is sent as given,
// 0 included — unlike Shrink, which floors it at 1 MB. Every form was run
// against SQL Server 2025 with each wait clause, TRUNCATEONLY included.
func TestScriptShrinkWrites(t *testing.T) {
	runScriptCases(t, []scriptCase{
		{"Database Shrink bare", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{})
		}, scriptUsePrefix + "DBCC SHRINKDATABASE (N'App''DB')"},
		{"Database Shrink with a target percent", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{TargetFreePercent: new(10)})
		}, scriptUsePrefix + "DBCC SHRINKDATABASE (N'App''DB', 10)"},
		{"Database Shrink to 0 percent", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{TargetFreePercent: new(0)})
		}, scriptUsePrefix + "DBCC SHRINKDATABASE (N'App''DB', 0)"},
		{"Database Shrink TRUNCATEONLY at low priority", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{Truncate: ShrinkTruncateOnly, Wait: ShrinkWaitLowPrioritySelf})
		}, scriptUsePrefix + "DBCC SHRINKDATABASE (N'App''DB', TRUNCATEONLY) WITH WAIT_AT_LOW_PRIORITY (ABORT_AFTER_WAIT = SELF)"},
		{"Database Shrink NOTRUNCATE killing blockers", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{TargetFreePercent: new(5), Truncate: ShrinkNoTruncate, Wait: ShrinkWaitLowPriorityBlockers})
		}, scriptUsePrefix + "DBCC SHRINKDATABASE (N'App''DB', 5, NOTRUNCATE) WITH WAIT_AT_LOW_PRIORITY (ABORT_AFTER_WAIT = BLOCKERS)"},
		{"File ShrinkWith TRUNCATEONLY", func(c context.Context) error {
			return scriptTestDB().FileRef("App'Dat]2").ShrinkWith(c, ShrinkFileOptions{Truncate: ShrinkTruncateOnly})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'App''Dat]2', TRUNCATEONLY)"},
		{"File ShrinkWith a target", func(c context.Context) error {
			return scriptTestDB().FileRef("AppDat").ShrinkWith(c, ShrinkFileOptions{TargetMB: new(int64(72))})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'AppDat', 72)"},
		{"File ShrinkWith target 0 is sent verbatim", func(c context.Context) error {
			return scriptTestDB().FileRef("AppLog").ShrinkWith(c, ShrinkFileOptions{TargetMB: new(int64(0))})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'AppLog', 0)"},
		{"FILESTREAM container ShrinkWith EMPTYFILE", func(c context.Context) error {
			return filestreamFile().ShrinkWith(c, ShrinkFileOptions{EmptyFile: true})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'fs1', EMPTYFILE)"},
		{"FileRef with no type sends TRUNCATEONLY", func(c context.Context) error {
			return scriptTestDB().FileRef("fs1").ShrinkWith(c, ShrinkFileOptions{Truncate: ShrinkTruncateOnly})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'fs1', TRUNCATEONLY)"},
		{"File ShrinkWith EMPTYFILE at low priority", func(c context.Context) error {
			return scriptTestDB().FileRef("f3").ShrinkWith(c, ShrinkFileOptions{EmptyFile: true, Wait: ShrinkWaitLowPriorityBlockers})
		}, scriptUsePrefix + "DBCC SHRINKFILE (N'f3', EMPTYFILE) WITH WAIT_AT_LOW_PRIORITY (ABORT_AFTER_WAIT = BLOCKERS)"},
	})
}

// filestreamFile is a FILESTREAM container as Database.Files returns one.
func filestreamFile() *DatabaseFileInfo {
	f := scriptTestDB().FileRef("fs1")
	f.Type = "FILESTREAM"
	return f
}

// TestShrinkRefusals pins the argument checks and the version gate: each is
// refused before anything is captured or sent.
func TestShrinkRefusals(t *testing.T) {
	old := &Database{server: &Server{info: &ServerInfo{VersionMajor: 15}}, Name: "AppDB"}
	azure := &Database{server: &Server{info: &ServerInfo{VersionMajor: 12, EngineEdition: int(EngineAzureManagedInst)}}, Name: "AppDB"}
	cases := []struct {
		name string
		call func(context.Context) error
		want error
	}{
		{"percent above 99", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{TargetFreePercent: new(100)})
		}, ErrInvalidRequest},
		{"negative percent", func(c context.Context) error {
			return scriptTestDB().Shrink(c, ShrinkDatabaseOptions{TargetFreePercent: new(-1)})
		}, ErrInvalidRequest},
		{"negative file target", func(c context.Context) error {
			return scriptTestDB().FileRef("f").ShrinkWith(c, ShrinkFileOptions{TargetMB: new(int64(-1))})
		}, ErrInvalidRequest},
		{"EMPTYFILE with a target", func(c context.Context) error {
			return scriptTestDB().FileRef("f").ShrinkWith(c, ShrinkFileOptions{EmptyFile: true, TargetMB: new(int64(1))})
		}, ErrInvalidRequest},
		{"EMPTYFILE with TRUNCATEONLY", func(c context.Context) error {
			return scriptTestDB().FileRef("f").ShrinkWith(c, ShrinkFileOptions{EmptyFile: true, Truncate: ShrinkTruncateOnly})
		}, ErrInvalidRequest},
		{"FILESTREAM container TRUNCATEONLY", func(c context.Context) error {
			return filestreamFile().ShrinkWith(c, ShrinkFileOptions{Truncate: ShrinkTruncateOnly})
		}, ErrInvalidRequest},
		{"FILESTREAM container NOTRUNCATE", func(c context.Context) error {
			return filestreamFile().ShrinkWith(c, ShrinkFileOptions{Truncate: ShrinkNoTruncate})
		}, ErrInvalidRequest},
		{"FILESTREAM container with a target", func(c context.Context) error {
			return filestreamFile().ShrinkWith(c, ShrinkFileOptions{TargetMB: new(int64(0))})
		}, ErrInvalidRequest},
		{"FILESTREAM container bare", func(c context.Context) error {
			return filestreamFile().ShrinkWith(c, ShrinkFileOptions{})
		}, ErrInvalidRequest},
		{"FILESTREAM container narrow Shrink", func(c context.Context) error {
			_, err := filestreamFile().Shrink(c, 0)
			return err
		}, ErrInvalidRequest},
		{"low priority on SQL Server 2019", func(c context.Context) error {
			return old.Shrink(c, ShrinkDatabaseOptions{Wait: ShrinkWaitLowPrioritySelf})
		}, ErrUnsupportedVersion},
		{"low priority file shrink on SQL Server 2019", func(c context.Context) error {
			return old.FileRef("f").ShrinkWith(c, ShrinkFileOptions{Wait: ShrinkWaitLowPriorityBlockers})
		}, ErrUnsupportedVersion},
		{"low priority on Azure is allowed", func(c context.Context) error {
			return azure.Shrink(c, ShrinkDatabaseOptions{Wait: ShrinkWaitLowPrioritySelf})
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, script := WithScript(context.Background())
			err := c.call(ctx)
			if c.want == nil {
				if err != nil {
					t.Fatalf("err = %v, want none", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if !strings.HasPrefix(err.Error(), "gosmo: shrink ") {
				t.Errorf("err = %q, want the gosmo: shrink prefix", err)
			}
			if n := len(script.Statements()); n != 0 {
				t.Errorf("captured %d statements, want none: %q", n, script.Statements())
			}
		})
	}
}

// TestFilesReadSpaceUsed pins SpaceUsedKB: FILEPROPERTY's SpaceUsed is a
// count of 8 KB pages, so the column is multiplied out in the query and
// scanned as KB, and a NULL (no permission, a FILESTREAM container) reads as
// 0, unknown.
func TestFilesReadSpaceUsed(t *testing.T) {
	cols := []string{"file_id", "name", "physical_name", "type_desc", "filegroup", "state_desc",
		"size", "max_size", "growth", "is_percent_growth", "space_used"}
	rows := [][]driver.Value{
		{int64(1), "appdb", `C:\data\appdb.mdf`, "ROWS", "PRIMARY", "ONLINE", int64(73728), int64(-1), int64(8192), false, int64(19648)},
		{int64(2), "appdb_log", `C:\data\appdb_log.ldf`, "LOG", "", "ONLINE", int64(73728), int64(-1), int64(8192), false, int64(51064)},
	}
	d := qsRecDB(t, 17, cols, rows)
	files, err := d.Files(context.Background())
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) != 2 || files[0].SpaceUsedKB != 19648 || files[1].SpaceUsedKB != 51064 {
		t.Fatalf("files = %+v, want SpaceUsedKB 19648 and 51064", files)
	}
	q := qsRec.last(t).sql
	if !strings.Contains(q, "ISNULL(CAST(FILEPROPERTY(df.name, 'SpaceUsed') AS bigint), 0) * 8") {
		t.Errorf("Files query does not read FILEPROPERTY SpaceUsed in KB:\n%s", q)
	}
}

func TestSupportsShrinkLowPriority(t *testing.T) {
	for _, c := range []struct {
		info *ServerInfo
		want bool
	}{
		{nil, true},
		{&ServerInfo{VersionMajor: 15}, false},
		{&ServerInfo{VersionMajor: 16}, true},
		{&ServerInfo{VersionMajor: 17}, true},
		{&ServerInfo{VersionMajor: 12, EngineEdition: int(EngineAzureManagedInst)}, true},
	} {
		if got := c.info.SupportsShrinkLowPriority(); got != c.want {
			t.Errorf("%+v: SupportsShrinkLowPriority = %v, want %v", c.info, got, c.want)
		}
	}
}
