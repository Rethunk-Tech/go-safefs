//go:build linux

package safefs

import (
	"strings"
	"testing"
)

func FuzzUnescapeMountPath(f *testing.F) {
	for _, s := range []string{
		"/home/user",
		`/home/user\040name`,
		`/tmp/\011tab`,
		`/var\012log`,
		`\134escaped`,
		`\040\011\012\134`,
		`\04`,
		`\`,
		`\400`,
		`\000`,
		"",
		`/run/user/1000/gvfs/smb-share:server=host,share=My\040Folder`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := unescapeMountPath(s)
		if len(got) > len(s) {
			t.Fatalf("unescapeMountPath grew %d to %d", len(s), len(got))
		}
		if round := unescapeMountPath(escapeMountPath(s)); round != s {
			t.Fatalf("unescape(escape(%q)) = %q", s, round)
		}
	})
}

// escapeMountPath is the kernel's mountinfo encoding: space, tab, newline, backslash as octal.
func escapeMountPath(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ':
			b.WriteString(`\040`)
		case '\t':
			b.WriteString(`\011`)
		case '\n':
			b.WriteString(`\012`)
		case '\\':
			b.WriteString(`\134`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
