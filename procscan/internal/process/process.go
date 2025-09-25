package process

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"log"
	"os"
	"path/filepath"
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
	log.Printf("开始分析进程 PID: %d", pid)

	procDir := filepath.Join(p.ProcPath, strconv.Itoa(pid))
	cmdlineFile := filepath.Join(procDir, "cmdline")

	log.Printf("读取进程 %d 的 cmdline 文件: %s", pid, cmdlineFile)
	cmdlineData, err := os.ReadFile(cmdlineFile)
	if err != nil {
		log.Printf("读取进程 %d cmdline 失败: %v", pid, err)
		return nil, err
	}

	cmdline := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)
	log.Printf("进程 %d 原始命令行: %q", pid, cmdline)

	if cmdline == "" {
		log.Printf("进程 %d 命令行为空，跳过", pid)
		return nil, nil
	}

	processName := p.getProcessName(cmdline)
	log.Printf("进程 %d 提取的进程名: %s", pid, processName)

	flag, msg := p.isMaliciousProcess(processName, cmdline)
	log.Printf("进程 %d 恶意检测结果: flag=%t, msg=%s", pid, flag, msg)

	if !flag {
		log.Printf("进程 %d 未检测到恶意行为，跳过", pid)
		return nil, nil
	}

	log.Printf("进程 %d 检测到恶意行为，开始获取容器ID", pid)
	containerID := p.getContainerIDFromPID(pid)
	log.Printf("进程 %d 获取到的容器ID: %s", pid, containerID)

	processInfo := &models.ProcessInfo{
		PID:         pid,
		ProcessName: processName,
		Command:     cmdline,
		Timestamp:   time.Now().Format(time.RFC3339),
		ContainerID: containerID,
		Message:     msg,
	}

	log.Printf("进程 %d 分析完成，返回结果: %+v", pid, processInfo)
	return processInfo, nil
}

func (p *Processor) getProcessName(cmdline string) string {
	log.Printf("提取进程名，输入命令行: %q", cmdline)

	parts := strings.Fields(cmdline)
	log.Printf("命令行分割结果: %v", parts)

	if len(parts) == 0 {
		log.Printf("命令行分割后为空，返回空字符串")
		return ""
	}

	execPath := parts[0]
	processName := filepath.Base(execPath)
	log.Printf("执行路径: %s, 提取的进程名: %s", execPath, processName)

	return processName
}

func (p *Processor) isMaliciousProcess(processName, cmdline string) (bool, string) {
	log.Printf("开始恶意进程检测，进程名: %s, 命令行: %q", processName, cmdline)

	lowerProcessName := strings.ToLower(processName)
	lowerCmdline := strings.ToLower(cmdline)
	log.Printf("转换为小写 - 进程名: %s, 命令行: %q", lowerProcessName, lowerCmdline)

	// 检查进程名和命令行中的禁用进程
	log.Printf("检查禁用进程列表: %v", p.Processes)
	for _, banned := range p.Processes {
		lowerBanned := strings.ToLower(banned)
		log.Printf("检查禁用进程: %s (小写: %s)", banned, lowerBanned)

		if strings.Contains(lowerProcessName, lowerBanned) {
			msg := fmt.Sprintf("进程名匹配禁用进程: %s", banned)
			log.Printf("匹配成功！%s", msg)
			return true, msg
		}

		if strings.Contains(lowerCmdline, lowerBanned) {
			msg := fmt.Sprintf("命令行匹配禁用进程: %s", banned)
			log.Printf("匹配成功！%s", msg)
			return true, msg
		}
	}

	// 检查命令行中的关键词
	log.Printf("检查关键词列表: %v", p.Keywords)
	for _, keyword := range p.Keywords {
		lowerKeyword := strings.ToLower(keyword)
		log.Printf("检查关键词: %s (小写: %s)", keyword, lowerKeyword)

		if strings.Contains(lowerCmdline, lowerKeyword) {
			msg := fmt.Sprintf("命令行匹配关键词: %s", keyword)
			log.Printf("匹配成功！%s", msg)
			return true, msg
		}
	}

	log.Printf("未检测到恶意行为")
	return false, ""
}

func (p *Processor) getContainerIDFromPID(pid int) string {
	log.Printf("开始获取进程 %d 的容器ID", pid)

	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	log.Printf("读取 cgroup 文件: %s", cgroupPath)

	content, err := os.ReadFile(cgroupPath)
	if err != nil {
		log.Printf("读取 cgroup 文件失败: %v", err)
		return ""
	}

	log.Printf("cgroup 文件内容:\n%s", string(content))

	lines := strings.Split(string(content), "\n")
	log.Printf("cgroup 文件共 %d 行", len(lines))

	for i, line := range lines {
		log.Printf("处理第 %d 行: %q", i+1, line)

		if strings.Contains(line, "containerd") || strings.Contains(line, "docker") {
			log.Printf("找到容器相关行: %s", line)

			parts := strings.Split(line, "/")
			log.Printf("路径分割结果: %v", parts)

			for j, part := range parts {
				log.Printf("检查第 %d 个部分: %q (长度: %d)", j+1, part, len(part))

				if len(part) == 64 {
					log.Printf("长度为64的部分，检查是否为十六进制")
					if isHexString(part) {
						log.Printf("找到容器ID: %s", part)
						return part
					} else {
						log.Printf("不是有效的十六进制字符串")
					}
				}
			}
		} else {
			log.Printf("跳过非容器相关行")
		}
	}

	log.Printf("未找到容器ID，返回空字符串")
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
