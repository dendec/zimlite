// Package memory reads Linux physical-memory diagnostics without inspecting
// virtual address reservations or depending on a rendering engine.
package memory

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type System struct {
	TotalKiB, AvailableKiB, SwapTotalKiB, SwapFreeKiB int64
}

type Process struct {
	RSSKiB, SwapKiB, PeakKiB int64
}

func ReadSystem() (System, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return System{}, err
	}
	return parseSystem(data)
}

func ReadProcess(pid int) (Process, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return Process{}, err
	}
	values, err := parseKiB(data)
	if err != nil {
		return Process{}, err
	}
	if _, ok := values["VmRSS"]; !ok {
		return Process{}, errors.New("process RSS unavailable")
	}
	return Process{RSSKiB: values["VmRSS"], SwapKiB: values["VmSwap"], PeakKiB: values["VmHWM"]}, nil
}

func parseSystem(data []byte) (System, error) {
	values, err := parseKiB(data)
	if err != nil {
		return System{}, err
	}
	total, totalOK := values["MemTotal"]
	available, availableOK := values["MemAvailable"]
	if !totalOK || !availableOK || total <= 0 || available > total {
		return System{}, errors.New("physical memory information unavailable")
	}
	return System{TotalKiB: total, AvailableKiB: available, SwapTotalKiB: values["SwapTotal"], SwapFreeKiB: values["SwapFree"]}, nil
}

func parseKiB(data []byte) (map[string]int64, error) {
	values := make(map[string]int64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid memory field %s", fields[0])
		}
		values[strings.TrimSuffix(fields[0], ":")] = n
	}
	return values, nil
}
