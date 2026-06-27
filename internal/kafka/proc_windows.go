//go:build windows

package kafka

import "syscall"

// CREATE_NO_WINDOW prevents spawned .bat/cmd processes from popping up a
// console window; their output is captured via pipes instead.
const createNoWindow = 0x08000000

// hiddenWindow returns SysProcAttr that suppresses the console window.
func hiddenWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
