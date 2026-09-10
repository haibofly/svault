package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle   = kernel32.NewProc("GetStdHandle")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	handleStdInput  = ^uintptr(0) - 9 // (DWORD)-10, STD_INPUT_HANDLE
	enableEchoInput = 0x0004
)

var stdinReader = bufio.NewReader(os.Stdin)

func readLineStdin() (string, error) {
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// stdinIsConsole reports whether stdin is attached to a console
// (as opposed to a pipe or redirected file).
func stdinIsConsole() bool {
	h, _, _ := procGetStdHandle.Call(handleStdInput)
	if h == 0 || h == uintptr(syscall.InvalidHandle) {
		return false
	}
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	return r != 0
}

// readSecret prompts for input and hides echo when stdin is a console.
// When stdin is redirected (pipe/file) it reads a plain line instead.
func readSecret(prompt string) (string, error) {
	h, _, _ := procGetStdHandle.Call(handleStdInput)
	if h == 0 || h == uintptr(syscall.InvalidHandle) {
		return readLineStdin()
	}
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode))); r == 0 {
		return readLineStdin()
	}

	fmt.Fprint(os.Stderr, prompt)
	procSetConsoleMode.Call(h, uintptr(mode&^enableEchoInput))
	defer procSetConsoleMode.Call(h, uintptr(mode))

	line, err := readLineStdin()
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return line, nil
}
