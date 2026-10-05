package runtime_monitor

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type processSample struct {
	RSS     uint64
	OpenFDs int
	Threads int
}

func collectProcessMetrics() processSample {
	return processSample{
		RSS:     readRSS(),
		OpenFDs: countFDs(),
		Threads: readThreads(),
	}
}

func readRSS() uint64 {
	if rssKB, ok := readLinuxStatusUint("VmRSS:"); ok {
		return rssKB * 1024
	}

	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}

	rss := uint64(ru.Maxrss)
	// Linux Maxrss 是 KB；Darwin 是 bytes。这里只保证生产 Linux 的 RSS 正确。
	if runtime.GOOS == "linux" || runtime.GOOS == "android" {
		rss *= 1024
	}
	return rss
}

func readThreads() int {
	if n, ok := readLinuxStatusUint("Threads:"); ok {
		return int(n)
	}
	return 0
}

func readLinuxStatusUint(prefix string) (uint64, bool) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func countFDs() int {
	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		return len(entries)
	}
	return 0
}
