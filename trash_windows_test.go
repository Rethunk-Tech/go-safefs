package safefs

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShFileOpErrorReadsItsOwnCodesBeforeWin32s(t *testing.T) {
	for _, c := range []struct {
		code       uintptr
		text       string
		inUse, per bool
		notExist   bool
	}{
		{code: 0x78, text: "denied access", inUse: true, per: true},
		{code: 0x7C, text: "may have a file in it open", inUse: true},
		{code: 0x75, text: "cancelled"},
		{code: 0x86, text: "read-only CD", per: true},
		{code: 0x10074, text: "root of a drive"},
		{code: 32, text: "used by another process", inUse: true},
		{code: 5, text: "denied", inUse: true, per: true},
		{code: 2, text: "cannot find", notExist: true},
	} {
		err := shFileOpError(c.code)
		if !strings.Contains(err.Error(), c.text) || errors.Is(err, ErrInUse) != c.inUse ||
			errors.Is(err, fs.ErrPermission) != c.per || errors.Is(err, fs.ErrNotExist) != c.notExist {
			t.Errorf("code 0x%X: %q, in use %v, permission %v, not exist %v", c.code, err,
				errors.Is(err, ErrInUse), errors.Is(err, fs.ErrPermission), errors.Is(err, fs.ErrNotExist))
		}
	}
}

// holdOpen opens file from a PowerShell child, without delete sharing, and makes its folder the child's current one
// until the test ends.
func holdOpen(t *testing.T, file string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"$f = [IO.File]::Open($env:HOLD, 'Open', 'Read', 'ReadWrite'); [Environment]::CurrentDirectory = Split-Path $env:HOLD; 'ready'; Start-Sleep 60")
	cmd.Env = append(os.Environ(), "HOLD="+file)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("holder said %q, %v", line, err)
	}
	return cmd
}

func TestTrashNamesTheProgramThatHoldsAFileInTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Mod²")
	file := filepath.Join(dir, "Plugin.dll")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := holdOpen(t, file)
	if err := Trash(dir); !errors.Is(err, ErrInUse) {
		t.Errorf("Trash of a held folder: %v", err)
	}
	for _, paths := range [][]string{{file, filepath.Join(dir, "missing.txt")}, {dir}} {
		holders, err := Holders(paths)
		if err != nil || len(holders) != 1 || holders[0].PID != child.Process.Pid || holders[0].Name == "" {
			t.Fatalf("holders of %v: %+v, want pid %d named (%v)", paths, holders, child.Process.Pid, err)
		}
		t.Logf("holders of %v: %+v", paths, holders)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	if err := Trash(dir); err != nil {
		t.Errorf("Trash once the holder is gone: %v", err)
	}
}
