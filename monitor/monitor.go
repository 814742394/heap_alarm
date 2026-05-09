package monitor

import (
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// FindProcess returns the first process whose name matches the given name.
// The matching is case-insensitive and the .exe suffix is optional.
func FindProcess(name string) (*process.Process, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	for _, p := range procs {
		procName, err := p.Name()
		if err != nil {
			continue
		}
		if equalProcessName(procName, name) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("process %q not found", name)
}

// FindAllProcesses returns all processes whose name matches the given name.
func FindAllProcesses(name string) ([]*process.Process, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	var result []*process.Process
	for _, p := range procs {
		procName, err := p.Name()
		if err != nil {
			continue
		}
		if equalProcessName(procName, name) {
			result = append(result, p)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("process %q not found", name)
	}
	return result, nil
}

// GetMemoryMB returns the process's resident set size (RSS) / working set in MB.
func GetMemoryMB(p *process.Process) (float64, error) {
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return 0, fmt.Errorf("get memory info for PID %d: %w", p.Pid, err)
	}
	// RSS on Windows is the working set size (private + shared).
	return float64(memInfo.RSS) / 1024 / 1024, nil
}

// equalProcessName compares two process names case-insensitively,
// tolerating a missing ".exe" suffix on either side.
func equalProcessName(a, b string) bool {
	a = stringsToLowerExe(a)
	b = stringsToLowerExe(b)
	return a == b
}

func stringsToLowerExe(s string) string {
	s = strings.ToLower(s)
	if len(s) > 4 && s[len(s)-4:] == ".exe" {
		s = s[:len(s)-4]
	}
	return s
}
