package ops

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func detectRAM() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 16 * 1024 * 1024 * 1024
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "MemTotal:") {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 16 * 1024 * 1024 * 1024
}
