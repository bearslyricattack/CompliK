// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ProcessStatus represents the status information from /proc/{pid}/status
type ProcessStatus struct {
	PID    int
	PPID   int
	NSpid  []int
	Name   string
	Tgid   int
	Tracer int
}

// ReadProcessStatus reads and parses /proc/{pid}/status file
func ReadProcessStatus(procPath string, pid int) (*ProcessStatus, error) {
	statusFile := fmt.Sprintf("%s/%d/status", procPath, pid)
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read status file: %w", err)
	}

	status := &ProcessStatus{PID: pid}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		switch key {
		case "Name":
			status.Name = fields[1]
		case "PPid":
			if ppid, err := strconv.Atoi(fields[1]); err == nil {
				status.PPID = ppid
			}
		case "Tgid":
			if tgid, err := strconv.Atoi(fields[1]); err == nil {
				status.Tgid = tgid
			}
		case "TracerPid":
			if tracer, err := strconv.Atoi(fields[1]); err == nil {
				status.Tracer = tracer
			}
		case "NSpid":
			// NSpid contains PIDs in different namespaces
			// Format: NSpid: <host_ns_pid> <container_ns_pid>
			nspids := make([]int, 0, len(fields)-1)
			for i := 1; i < len(fields); i++ {
				if nspid, err := strconv.Atoi(fields[i]); err == nil {
					nspids = append(nspids, nspid)
				}
			}
			status.NSpid = nspids
		}
	}

	return status, nil
}

// IsContainerMainProcess determines if a process is the main process of a container
// by checking if its PID in the container namespace (NSpid) is 1
func IsContainerMainProcess(status *ProcessStatus) bool {
	if status == nil || len(status.NSpid) < 2 {
		return false
	}
	// The last NSpid value represents the PID within the innermost namespace (container)
	containerPID := status.NSpid[len(status.NSpid)-1]
	return containerPID == 1
}

// FindContainerMainProcess traces back from the given PID to find the container's main process
// Returns the PID of the main process, or 0 if not found
func FindContainerMainProcess(procPath string, pid int) (int, error) {
	visited := make(map[int]bool)
	currentPID := pid

	for {
		if visited[currentPID] {
			// Circular reference detected (shouldn't happen in normal cases)
			return 0, fmt.Errorf("circular process parent reference detected at PID %d", currentPID)
		}
		visited[currentPID] = true

		status, err := ReadProcessStatus(procPath, currentPID)
		if err != nil {
			// If we can't read the status, the process might have exited
			return 0, fmt.Errorf("failed to read process status for PID %d: %w", currentPID, err)
		}

		// Check if this is the container main process
		if IsContainerMainProcess(status) {
			return currentPID, nil
		}

		// Move to parent process
		if status.PPID == 0 || status.PPID == currentPID {
			// Reached init process or circular reference
			return 0, fmt.Errorf("reached init process without finding container main process")
		}

		currentPID = status.PPID
	}
}

// GetProcessNamespaceInfo extracts namespace information for better process identification
func GetProcessNamespaceInfo(procPath string, pid int) (map[string]string, error) {
	nsPath := fmt.Sprintf("%s/%d/ns", procPath, pid)
	entries, err := os.ReadDir(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace directory: %w", err)
	}

	nsInfo := make(map[string]string)
	for _, entry := range entries {
		linkPath := fmt.Sprintf("%s/%s", nsPath, entry.Name())
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}
		nsInfo[entry.Name()] = target
	}

	return nsInfo, nil
}
