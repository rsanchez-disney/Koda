package ops

import (
	"os/exec"
	"strconv"
	"strings"
)

func detectRAM() uint64 {
	out, err := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory", "/value").Output()
	if err != nil {
		return 16 * 1024 * 1024 * 1024
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "TotalPhysicalMemory=") {
			val := strings.TrimPrefix(strings.TrimSpace(line), "TotalPhysicalMemory=")
			n, _ := strconv.ParseUint(strings.TrimSpace(val), 10, 64)
			if n > 0 {
				return n
			}
		}
	}
	return 16 * 1024 * 1024 * 1024
}
