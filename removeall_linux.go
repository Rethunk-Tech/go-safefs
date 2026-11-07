//go:build linux

package safefs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// mountsBelow lists the mount points strictly below path in this process's mount namespace.
func mountsBelow(path string) ([]string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(parent, filepath.Base(abs))
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var found []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		mount := unescapeMountPath(fields[4])
		if mount != dir && strings.HasPrefix(mount, strings.TrimSuffix(dir, "/")+"/") {
			found = append(found, mount)
		}
	}
	return found, sc.Err()
}

// unescapeMountPath reverses the kernel's octal escapes (\040 for a space) in a mountinfo path.
func unescapeMountPath(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
