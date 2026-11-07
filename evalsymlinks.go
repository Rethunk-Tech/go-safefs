package safefs

import "strings"

// ExtendedPath is the \\?\ form of an absolute Windows path of drive-letter form that is too long for the
// classic API, which filepath.EvalSymlinks and raw CreateFile calls use without any prefixing of their own. Short,
// UNC and already-extended paths come back unchanged.
func ExtendedPath(abs string) string {
	if len(abs) < 248 || strings.HasPrefix(abs, `\\`) {
		return abs
	}
	return `\\?\` + abs
}

// trimExtended turns an EvalSymlinks result of an extended-length path back into the classic form: \\?\C:\x
// becomes C:\x and \\?\UNC\server\share becomes \\server\share. Anything else is returned unchanged.
func trimExtended(resolved string) string {
	if rest, ok := strings.CutPrefix(resolved, `\\?\UNC\`); ok {
		return `\\` + rest
	}
	return strings.TrimPrefix(resolved, `\\?\`)
}
