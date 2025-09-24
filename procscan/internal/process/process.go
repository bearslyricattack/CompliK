package process

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Processor struct {
	ProcPath  string
	Processes []string
	Keywords  []string
}

func NewProcessor(config *models.Config) *Processor {
	return &Processor{
		ProcPath:  config.ProcPath,
		Processes: config.Processes,
		Keywords:  config.Keywords,
	}
}

func (p *Processor) GetAllProcesses() ([]int, error) {
	procDirs, err := os.ReadDir(p.ProcPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 目录失败: %w", p.ProcPath, err)
	}
	pids := make([]int, 0, len(procDirs))
	for _, dir := range procDirs {
		if !dir.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(dir.Name())
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func (p *Processor) AnalyzeProcess(pid int) (*models.ProcessInfo, error) {
	procDir := filepath.Join(p.ProcPath, strconv.Itoa(pid))
	cmdlineFile := filepath.Join(procDir, "cmdline")
	cmdlineData, err := os.ReadFile(cmdlineFile)
	if err != nil {
		return nil, err
	}
	cmdline := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return nil, nil
	}
	processName := p.getProcessName(cmdline)
	flag, msg := p.isMaliciousProcess(processName, cmdline)
	if !flag {
		return nil, nil
	}
	containerID := p.getContainerIDFromPID(pid)
	processInfo := &models.ProcessInfo{
		PID:         pid,
		ProcessName: processName,
		Command:     cmdline,
		Timestamp:   time.Now().Format(time.RFC3339),
		ContainerID: containerID,
		Message:     msg,
	}
	return processInfo, nil
}

func (p *Processor) getProcessName(cmdline string) string {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return ""
	}
	execPath := parts[0]
	return filepath.Base(execPath)
}

func (p *Processor) isMaliciousProcess(processName, cmdline string) (bool, string) {
	lowerProcessName := strings.ToLower(processName)
	lowerCmdline := strings.ToLower(cmdline)

	// 检查进程名和命令行中的禁用进程
	for _, banned := range p.Processes {
		lowerBanned := strings.ToLower(banned)

		// 进程名精确匹配或作为单词匹配
		if lowerProcessName == lowerBanned || strings.Contains(lowerProcessName, lowerBanned) {
			return true, fmt.Sprintf("进程名匹配禁用进程: %s", banned)
		}

		// 命令行中作为独立单词匹配
		pattern := `\b` + regexp.QuoteMeta(lowerBanned) + `\b`
		matched, _ := regexp.MatchString(pattern, lowerCmdline)
		if matched {
			return true, fmt.Sprintf("命令行匹配禁用进程: %s", banned)
		}
	}

	// 检查命令行中的关键词
	for _, keyword := range p.Keywords {
		lowerKeyword := strings.ToLower(keyword)
		pattern := `\b` + regexp.QuoteMeta(lowerKeyword) + `\b`
		matched, _ := regexp.MatchString(pattern, lowerCmdline)
		if matched {
			return true, fmt.Sprintf("命令行匹配关键词: %s", keyword)
		}
	}

	return false, ""
}

func (p *Processor) getContainerIDFromPID(pid int) string {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	content, err := os.ReadFile(cgroupPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "containerd") || strings.Contains(line, "docker") {
			parts := strings.Split(line, "/")
			for _, part := range parts {
				if len(part) == 64 && isHexString(part) {
					return part
				}
			}
		}
	}
	return ""
}

func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func (p *Processor) UpdateConfig(config *models.Config) {
	p.Processes = config.Processes
	p.Keywords = config.Keywords
}
