package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// pauseIfOwnWindow waits for Enter when the program has a console window to
// itself, as when the document is dropped onto it in Explorer: otherwise the
// window would close before the result could be read. Started from a command
// prompt, it shares the console and doesn't wait.
func pauseIfOwnWindow() {
	getConsoleProcessList := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n != 1 {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprint(os.Stderr, "Press Enter to close this window.")
	_, _ = fmt.Fscanln(os.Stdin)
}
