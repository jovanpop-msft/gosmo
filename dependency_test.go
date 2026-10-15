package gosmo

import (
	"context"
	"database/sql/driver"
	"errors"
	"slices"
	"testing"
)

// dependencyReads are the two directions of the dependency graph, which share
// their argument binding and scan through dependencyEdges.
var dependencyReads = []struct {
	name string
	read func(d *Database, schema, name string) ([]*Dependency, error)
}{
	{"Dependencies", func(d *Database, schema, name string) ([]*Dependency, error) {
		return d.Dependencies(context.Background(), schema, name)
	}},
	{"Dependents", func(d *Database, schema, name string) ([]*Dependency, error) {
		return d.Dependents(context.Background(), schema, name)
	}},
}

// TestDependencyReadsRefuseAnEmptySchema pins that neither read guesses a
// schema: OBJECT_ID of a one-part name resolves against the connection's
// default schema, which is not necessarily the caller's.
func TestDependencyReadsRefuseAnEmptySchema(t *testing.T) {
	for _, r := range dependencyReads {
		t.Run(r.name, func(t *testing.T) {
			d := qsRecDB(t, 17, nil, nil)
			if _, err := r.read(d, "", "orders"); !errors.Is(err, ErrSchemaRequired) {
				t.Fatalf("err = %v, want ErrSchemaRequired", err)
			}
			if calls := qsRec.recorded(); len(calls) != 0 {
				t.Errorf("a refused read reached the driver: %q", calls[0].sql)
			}
		})
	}
}

// TestDependencyReadsBindTheQuotedNameAndScanInOrder pins the one argument
// both queries take — the bracket-quoted two-part name OBJECT_ID needs, with
// a ] in it doubled — and the order of the two bit columns, which a swap
// would turn into schema-bound edges reported as caller-dependent.
func TestDependencyReadsBindTheQuotedNameAndScanInOrder(t *testing.T) {
	cols := []string{"schema", "name", "type_desc", "is_schema_bound", "is_caller_dependent"}
	rows := [][]driver.Value{
		{"dbo", "orders", "USER_TABLE", true, false},
		{"app", "get_order", "SQL_STORED_PROCEDURE", false, true},
	}
	want := []Dependency{
		{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE", IsSchemaBound: true},
		{Schema: "app", Name: "get_order", TypeDesc: "SQL_STORED_PROCEDURE", IsCallerDependent: true},
	}
	for _, r := range dependencyReads {
		t.Run(r.name, func(t *testing.T) {
			d := qsRecDB(t, 17, cols, rows)
			got, err := r.read(d, "sa]les", "v")
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if args := qsRec.last(t).args; !slices.Equal(args, []any{"[sa]]les].[v]"}) {
				t.Errorf("bound %v, want [[sa]]les].[v]]", args)
			}
			if len(got) != len(want) {
				t.Fatalf("got %d edges, want %d", len(got), len(want))
			}
			for i := range want {
				if *got[i] != want[i] {
					t.Errorf("edge %d = %+v, want %+v", i, *got[i], want[i])
				}
			}
		})
	}
}
