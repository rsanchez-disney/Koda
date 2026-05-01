package ops

import (
	"os/exec"
	"strconv"
	"strings"
)

func detectRAM() uint64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 16 * 1024 * 1024 * 1024 // default 16GB
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 16 * 1024 * 1024 * 1024
	}
	return n
}
