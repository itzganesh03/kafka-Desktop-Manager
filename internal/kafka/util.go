package kafka

import (
	"os/exec"
	"strconv"
	"strings"
)

// containsFold reports whether s contains substr, case-insensitively.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// killTree terminates a process and all of its children on Windows. The Kafka
// .bat scripts spawn a java.exe child, so killing only the cmd.exe parent would
// leave the broker running; taskkill /T handles the whole tree.
func killTree(pid int) error {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = hiddenWindow()
	return cmd.Run()
}
