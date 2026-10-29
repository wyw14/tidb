// Temporary debug.
package parser

import (
	"testing"
)

func TestQualifyLexDbg(t *testing.T) {
	p := New()
	for _, q := range []string{
		"select qualify from t",
		"select 1 qualify 1",
		"select 1 from t qualify 1",
		"select 1 from t having 1",
	} {
		stmts, _, err := p.Parse(q, "", "")
		if err != nil {
			t.Logf("ERR  %q -> %v", q, err)
			continue
		}
		t.Logf("OK   %q -> %#v", q, stmts[0])
	}
}
