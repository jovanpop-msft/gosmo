package gosmo

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================
// Shrink  (DBCC SHRINKDATABASE / DBCC SHRINKFILE — SSMS's Tasks > Shrink >
// Database and Files)
// ============================================================

// ShrinkTruncate is the release clause of a shrink: whether the space freed at
// the end of a file goes back to the operating system, and whether pages are
// moved to free it first.
type ShrinkTruncate int

const (
	// ShrinkTruncateDefault sends no clause: pages are moved towards the
	// start of the file, then the free space at the end is released.
	ShrinkTruncateDefault ShrinkTruncate = iota
	// ShrinkNoTruncate moves pages but keeps the freed space in the file.
	// Data files only; the server ignores it on a log.
	ShrinkNoTruncate
	// ShrinkTruncateOnly releases the free space at the end of the file
	// without moving a page — SSMS's "Release unused space". Any target is
	// ignored by the server.
	ShrinkTruncateOnly
)

// ShrinkWait is the WITH WAIT_AT_LOW_PRIORITY clause (SQL Server 2022 and
// Azure): the shrink waits for its schema-modification lock at low priority
// for a fixed minute instead of queueing ahead of every later query on the
// table, then gives up as ABORT_AFTER_WAIT says. The minute is not
// configurable — the server has no MAX_DURATION for a shrink.
type ShrinkWait int

const (
	// ShrinkWaitNormal sends no clause: the shrink waits for its locks the
	// ordinary way, blocking the queries queued behind it.
	ShrinkWaitNormal ShrinkWait = iota
	// ShrinkWaitLowPrioritySelf ends the shrink when the minute is up
	// (ABORT_AFTER_WAIT = SELF, the server's default).
	ShrinkWaitLowPrioritySelf
	// ShrinkWaitLowPriorityBlockers kills the transactions blocking the
	// shrink when the minute is up (ABORT_AFTER_WAIT = BLOCKERS).
	ShrinkWaitLowPriorityBlockers
)

// ShrinkDatabaseOptions shapes Database.Shrink's DBCC SHRINKDATABASE. The zero
// value is the bare statement SSMS scripts with "Reorganize files" unticked.
type ShrinkDatabaseOptions struct {
	// TargetFreePercent is the free space, 0–99, to leave in each file after
	// the shrink — SSMS's "Maximum free space in files after shrinking". nil
	// omits the argument.
	TargetFreePercent *int
	Truncate          ShrinkTruncate
	Wait              ShrinkWait
}

// ShrinkFileOptions shapes DatabaseFileInfo.ShrinkWith's DBCC SHRINKFILE.
//
// A FILESTREAM container takes EMPTYFILE and nothing else: any other form is
// Msg 2563. On a file whose Type reads "FILESTREAM" (one Database.Files
// returned) ShrinkWith refuses those forms itself; a FileRef has no type, so
// it sends them and the server answers.
type ShrinkFileOptions struct {
	// TargetMB is the size to shrink the file to, sent verbatim — 0 included,
	// which stops at the file's creation size rather than its contents (see
	// Shrink, which floors it at 1 instead). nil omits it: the file shrinks
	// to its creation size.
	TargetMB *int64
	Truncate ShrinkTruncate
	// EmptyFile moves every page of the file to the other files in its
	// filegroup (EMPTYFILE), so it can be removed. It takes neither a target
	// nor a release clause.
	EmptyFile bool
	Wait      ShrinkWait
}

// Shrink shrinks every file of the database (DBCC SHRINKDATABASE). It moves
// pages and so fragments the indexes it touches, and it is slow on a large
// database. Under WithScript it captures the statement.
func (d *Database) Shrink(ctx context.Context, opts ShrinkDatabaseOptions) error {
	stmt, err := buildShrinkDatabaseStatement(d.Name, opts, d.server)
	if err != nil {
		return fmt.Errorf("gosmo: shrink database %q: %w", d.Name, err)
	}
	if _, err := d.exec(ctx, stmt); err != nil {
		return fmt.Errorf("gosmo: shrink database %q: %w", d.Name, err)
	}
	return nil
}

// ShrinkWith shrinks the file (DBCC SHRINKFILE) with every option the
// command takes: a target sent as given, TRUNCATEONLY, NOTRUNCATE, EMPTYFILE
// and WAIT_AT_LOW_PRIORITY. Shrink is the narrower form that reports the size
// reached. Under WithScript it captures the statement.
func (f *DatabaseFileInfo) ShrinkWith(ctx context.Context, opts ShrinkFileOptions) error {
	stmt, err := buildShrinkFileWithStatement(f.Name, f.Type, opts, f.db.server)
	if err != nil {
		return fmt.Errorf("gosmo: shrink file %q in %q: %w", f.Name, f.db.Name, err)
	}
	if _, err := f.db.exec(ctx, stmt); err != nil {
		return fmt.Errorf("gosmo: shrink file %q in %q: %w", f.Name, f.db.Name, err)
	}
	return nil
}

// buildShrinkDatabaseStatement renders Database.Shrink's statement, naming the
// database as a literal, as SSMS does.
func buildShrinkDatabaseStatement(name string, opts ShrinkDatabaseOptions, s *Server) (string, error) {
	args := []string{QuoteLiteral(name)}
	if p := opts.TargetFreePercent; p != nil {
		if *p < 0 || *p > 99 {
			return "", invalidf("target free space %d%% is outside 0–99", *p)
		}
		args = append(args, fmt.Sprint(*p))
	}
	args, err := appendShrinkTruncate(args, opts.Truncate)
	if err != nil {
		return "", err
	}
	return shrinkStatement("DBCC SHRINKDATABASE", args, opts.Wait, s)
}

// buildShrinkFileWithStatement renders ShrinkWith's statement, naming the file
// by its logical name as a literal — the handle may be a FileRef, with no
// file_id. typeDesc is the file's Type as the handle holds it, "" when unknown:
// it is passed in, not read, so a FileRef stays lookup-free.
func buildShrinkFileWithStatement(name, typeDesc string, opts ShrinkFileOptions, s *Server) (string, error) {
	args := []string{QuoteLiteral(name)}
	if typeDesc == "FILESTREAM" && !opts.EmptyFile {
		return "", invalidf("a FILESTREAM container can only be emptied (EMPTYFILE)")
	}
	if opts.EmptyFile {
		if opts.TargetMB != nil || opts.Truncate != ShrinkTruncateDefault {
			return "", invalidf("EMPTYFILE takes neither a target size nor a release clause")
		}
		args = append(args, "EMPTYFILE")
		return shrinkStatement("DBCC SHRINKFILE", args, opts.Wait, s)
	}
	if t := opts.TargetMB; t != nil {
		if *t < 0 {
			return "", invalidf("target size %d MB is negative", *t)
		}
		args = append(args, fmt.Sprint(*t))
	}
	args, err := appendShrinkTruncate(args, opts.Truncate)
	if err != nil {
		return "", err
	}
	return shrinkStatement("DBCC SHRINKFILE", args, opts.Wait, s)
}

func appendShrinkTruncate(args []string, t ShrinkTruncate) ([]string, error) {
	switch t {
	case ShrinkTruncateDefault:
		return args, nil
	case ShrinkNoTruncate:
		return append(args, "NOTRUNCATE"), nil
	case ShrinkTruncateOnly:
		return append(args, "TRUNCATEONLY"), nil
	}
	return nil, invalidf("unknown release clause %d", t)
}

// shrinkStatement joins the command, its arguments and the wait clause. The
// wait clause is refused with ErrUnsupportedVersion before SQL Server 2022,
// where it is a syntax error; major 0 (never read, or Azure) passes.
func shrinkStatement(cmd string, args []string, wait ShrinkWait, s *Server) (string, error) {
	stmt := fmt.Sprintf("%s (%s)", cmd, strings.Join(args, ", "))
	var abort string
	switch wait {
	case ShrinkWaitNormal:
		return stmt, nil
	case ShrinkWaitLowPrioritySelf:
		abort = "SELF"
	case ShrinkWaitLowPriorityBlockers:
		abort = "BLOCKERS"
	default:
		return "", invalidf("unknown wait %d", wait)
	}
	if !s.info.SupportsShrinkLowPriority() {
		return "", unsupportedVersionf("WAIT_AT_LOW_PRIORITY needs SQL Server 2022 or later; this instance is major %d", s.serverMajorVersion())
	}
	return stmt + " WITH WAIT_AT_LOW_PRIORITY (ABORT_AFTER_WAIT = " + abort + ")", nil
}
