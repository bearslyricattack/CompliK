package process

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Processor 进程处理器
type Processor struct {
	ProcPath  string
	NodeName  string
	Processes []string
	Keywords  []string
}

func NewProcessor(config *models.Config) *Processor {
	return &Processor{
		ProcPath:  config.ProcPath,
		NodeName:  config.NodeName,
		Processes: config.Processes,
		Keywords:  config.Keywords,
	}
}

func (p *Processor) GetAllProcesses() ([]int, error) {
	procDirs, err := os.ReadDir(p.ProcPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 目录失败: %w", p.procPath, err)
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
	if !p.isMaliciousProcess(processName, cmdline) {
		return nil, nil
	}
	processInfo := &models.ProcessInfo{
		PID:         pid,
		ProcessName: processName,
		Command:     cmdline,
		NodeName:    p.NodeName,
		Timestamp:   time.Now().Format(time.RFC3339),
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

func (p *Processor) isMaliciousProcess(processName, cmdline string) bool {
	// 转换为小写进行比较
	lowerProcessName := strings.ToLower(processName)
	lowerCmdline := strings.ToLower(cmdline)

	// 检查禁用进程列表
	for _, banned := range p.Processes {
		if strings.Contains(lowerProcessName, strings.ToLower(banned)) ||
			strings.Contains(lowerCmdline, strings.ToLower(banned)) {
			return true
		}
	}
	// 检查关键词
	for _, keyword := range p.Keywords {
		if strings.Contains(lowerCmdline, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (p *Processor) UpdateConfig(config *models.Config) {
	p.Processes = config.Processes
	p.Keywords = config.Keywords
}
