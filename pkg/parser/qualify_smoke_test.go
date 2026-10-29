// Temporary smoke test for QUALIFY parsing. Delete after verification.
package parser

import (
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"
	f "github.com/pingcap/tidb/pkg/parser/format"
	trequire "github.com/stretchr/testify/require"
)

func restoreSQL(t *testing.T, stmt ast.StmtNode) string {
	var sb strings.Builder
	trequire.NoError(t, stmt.Restore(f.NewRestoreCtx(f.DefaultRestoreFlags, &sb)))
	return sb.String()
}

func TestQualifySmoke(t *testing.T) {
	p := New()
	cases := []struct {
		sql     string
		qualify bool
	}{
		{"select user_id, row_number() over (partition by team_id order by score desc) as rn from scores qualify rn <= 2", true},
		{"select user_id from scores qualify row_number() over (partition by team_id order by score desc) <= 2", true},
		{"select a, count(*) from t group by a having count(*) > 1 window w as (order by a) qualify sum(b) over w > 1 order by a limit 10", true},
		{"select 1 from dual qualify 1", true},
		{"select 1 qualify 1", true},
		{"select * from (select 1) as `qualify`", false}, // table alias named qualify (backtick)
		{"select * from `qualify`", false},
		{"select 1 as `qualify`", false},
		{"select * from t where score > 10 qualify rn <= 2 order by user_id desc limit 1", true},
	}
	for _, c := range cases {
		stmts, _, err := p.Parse(c.sql, "", "")
		trequire.NoError(t, err, c.sql)
		trequire.Len(t, stmts, 1, c.sql)
		sel, ok := stmts[0].(*ast.SelectStmt)
		trequire.True(t, ok, c.sql)
		if c.qualify {
			trequire.NotNil(t, sel.Qualify, c.sql)
		} else {
			trequire.Nil(t, sel.Qualify, c.sql)
		}
		sql := restoreSQL(t, stmts[0])
		t.Logf("RESTORED: %s", sql)
		stmts2, _, err := p.Parse(sql, "", "")
		trequire.NoError(t, err, sql)
		trequire.Equal(t, sql, restoreSQL(t, stmts2[0]))
		t.Logf("OK: %s => %s", c.sql, sql)
	}
	// QUALIFY must not be attachable to TABLE/VALUES statements
	_, _, err := p.Parse("table t qualify 1", "", "")
	trequire.Error(t, err)
	// reserved keyword can't be a bare identifier
	for _, q := range []string{"select 1 qualify", "select * from qualify", "select 1 as qualify"} {
		_, _, err = p.Parse(q, "", "")
		trequire.Error(t, err, q)
	}
}
