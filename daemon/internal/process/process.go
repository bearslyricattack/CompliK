package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/daemon/internal/types"
)

// Processor 进程处理器
type Processor struct {
	procPath string
	config   types.Config
	nodeName string
}

// NewProcessor 创建进程处理器
func NewProcessor(procPath, nodeName string, config types.Config) *Processor {
	return &Processor{
		procPath: procPath,
		config:   config,
		nodeName: nodeName,
	}
}

func (p *Processor) GetAllProcesses() ([]int, error) {
	procDirs, err := os.ReadDir(p.procPath)
	if err != nil {
		return nil, fmt.Errorf("读取 /proc 目录失败: %w", err)
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

func (p *Processor) AnalyzeProcess(pid int) (*types.ProcessInfo, error) {
	procDir := filepath.Join(p.procPath, strconv.Itoa(pid))
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
	processInfo := &types.ProcessInfo{
		PID:         pid,
		ProcessName: processName,
		Command:     cmdline,
		NodeName:    p.nodeName,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return processInfo, nil
}

// getProcessName 从命令行获取进程名
func (p *Processor) getProcessName(cmdline string) string {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return ""
	}

	// 获取可执行文件的基本名称
	execPath := parts[0]
	return filepath.Base(execPath)
}

// isMaliciousProcess 检查是否为恶意进程
func (p *Processor) isMaliciousProcess(processName, cmdline string) bool {
	// 转换为小写进行比较
	lowerProcessName := strings.ToLower(processName)
	lowerCmdline := strings.ToLower(cmdline)

	// 检查禁用进程列表
	for _, banned := range p.config.BannedProcesses {
		if strings.Contains(lowerProcessName, strings.ToLower(banned)) ||
			strings.Contains(lowerCmdline, strings.ToLower(banned)) {
			return true
		}
	}

	// 检查关键词
	for _, keyword := range p.config.Keywords {
		if strings.Contains(lowerCmdline, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// UpdateConfig 更新配置
func (p *Processor) UpdateConfig(config types.Config) {
	p.config = config
}
