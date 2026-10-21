package gosmo

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// G11 (review plan 2026-10-08): a call refused for its own arguments wraps
// ErrInvalidRequest, so a caller can tell "fix the request" from "the server
// said no" with errors.Is rather than by matching text. invalidf builds every
// such error.
//
// This keeps the rule from eroding: every fmt.Errorf whose format has no %w,
// and every errors.New inside a function — an error that wraps nothing, so
// one gosmo decided on its own — must be on this list, which is the refusals
// about the server's state rather than the caller's arguments (invalidf),
// gosmo having no form for the request (unsupportedf) or something absent
// (notFoundf). A new check written with fmt.Errorf or errors.New fails here.
// A package-level var ErrX = errors.New(...) is a sentinel, not a refusal,
// and is not scanned. An entry nothing matches any more fails too, so the
// list only shrinks with the code.
func TestEveryArgumentRefusalWrapsErrInvalidRequest(t *testing.T) {
	// File → the start of each allowed format string.
	allowed := map[string][]string{
		"catalog.go":        {"result set %d of %d is missing"},
		"certificate.go":    {"gosmo: encode certificate %q in %q: CERTENCODED returned nothing"},
		"connection_dsn.go": {"gosmo: connection string: cannot mask an unparseable DSN"},
		"database_mail.go":  {"gosmo: read mail status: unexpected status"},
		"database_users.go": {"gosmo: create user %q: a user with a password needs a contained database"},
		"entra.go": {
			"gosmo: no Entra credential mapping for fedauth",
			"the server completed a login without asking for an Entra token",
		},
		"errors.go":               {"", ""}, // invalidf and unsupportedf themselves
		"executionplan.go":        {"gosmo: capture execution plan: no plan was returned"},
		"extended_events_read.go": {"gosmo: %s: the server reports no error-log directory"},
		"helpers.go":              {"a result set names database %q, which was not asked for"},
		"index_management.go":     {"gosmo: partition %d listed at position %d"},
		"scripter_index.go":       {"partition scheme %s has no partitioning column"},
		"scripter_security.go":    {"gosmo: script database audit specification %q: it names no audit"},
		"scripter_server.go": {
			"gosmo: script endpoint %q: its database mirroring detail could not be read",
			"gosmo: script endpoint %q: its service broker detail could not be read",
			"gosmo: script server audit specification %q: it names no audit",
		},
		// Script mode's argument binding: the statement is fine, the capture
		// has no literal form for it.
		"script.go": {
			"gosmo: script: named argument %q cannot be scripted positionally",
			"gosmo: script: %s has no argument (%d given)",
			"gosmo: script: %s appears only inside a literal or comment",
			"named argument %q cannot be scripted positionally",
			"cannot script a %T argument",
		},
		"server_databases.go": {"a log file needs a data file beside it, and the instance reports no default data path"},
		"symmetric_key.go": {
			"symmetric key %q has no decryptor to open it with",
			"symmetric key %q: decryptor chain deeper than %d",
		},
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	seen := 0
	used := map[string]bool{} // file + "\x00" + allowed prefix
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		// Sentinels: errors.New as the value of a package-level var.
		sentinel := map[*ast.CallExpr]bool{}
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.VAR {
				for _, spec := range g.Specs {
					for _, v := range spec.(*ast.ValueSpec).Values {
						if call, ok := v.(*ast.CallExpr); ok {
							sentinel[call] = true
						}
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || sentinel[call] {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			fn := x.Name + "." + sel.Sel.Name
			if fn != "fmt.Errorf" && fn != "errors.New" {
				return true
			}
			format := literalText(call.Args[0])
			if fn == "fmt.Errorf" && strings.Contains(format, "%w") {
				return true
			}
			seen++
			for _, prefix := range allowed[name] {
				if strings.HasPrefix(format, prefix) {
					used[name+"\x00"+prefix] = true
					return true
				}
			}
			t.Errorf("%s: %s(%q) wraps nothing: use invalidf, unsupportedf or notFoundf, or add it to this test's list if it is about the server's state",
				fset.Position(call.Pos()), fn, format)
			return true
		})
	}
	if seen == 0 {
		t.Fatal("found no fmt.Errorf at all; the scan is broken")
	}
	for name, prefixes := range allowed {
		for _, prefix := range prefixes {
			if !used[name+"\x00"+prefix] {
				t.Errorf("%s: allowed %q matches no error any more; drop it from the list", name, prefix)
			}
		}
	}
}

// literalText is a format argument's text: a string literal, or a
// concatenation of them. Anything else reads as "", which only the errors.go
// entries match.
func literalText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(e.Value)
		if err == nil {
			return s
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return literalText(e.X) + literalText(e.Y)
		}
	}
	return ""
}

// A sample of refusals from different families, through the public API: each
// wraps ErrInvalidRequest and keeps its message.
func TestArgumentRefusalsWrapErrInvalidRequest(t *testing.T) {
	d, ctx, _ := scriptedDB(t)
	s := d.server
	cases := []struct {
		name string
		err  error
		msg  string
	}{
		{"create table", errOnly(d.CreateTable(ctx, CreateTableRequest{})), "gosmo: create table: name is required"},
		{"set recovery model", d.SetRecoveryModel(ctx, "NOPE"), `gosmo: set recovery model of "appdb": unrecognized recovery model "NOPE"`},
		{"create login", errOnly(s.CreateLogin(ctx, CreateLoginRequest{Name: "x", Source: LoginSourceSQL})), "a SQL login requires a password"},
		{"signer", d.StoredProcedureRef("dbo", "p").AddSignature(ctx, Signer{Kind: SignerCertificate}, PlainSignature), "signature by certificate has no name"},
		{"apply permission", d.ApplyPermission(ctx, VerbGrant, Securable{Class: SecurableTable, Schema: "dbo", Name: "t"}, PermExecute, "x", PermissionOptions{}),
			`gosmo: grant permission: "EXECUTE" cannot be granted on a table`},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, ErrInvalidRequest) {
			t.Errorf("%s: %v does not wrap ErrInvalidRequest", tc.name, tc.err)
		}
		if tc.err == nil || !strings.Contains(tc.err.Error(), tc.msg) {
			t.Errorf("%s: error %v, want it to read %q", tc.name, tc.err, tc.msg)
		}
		if errors.Is(tc.err, ErrUnsupported) {
			t.Errorf("%s: an argument refusal also reads as ErrUnsupported", tc.name)
		}
	}
}
