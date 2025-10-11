package process

import (
	"fmt"
	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/internal/container"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

type compiledRules struct {
	blacklistProcesses  []*regexp.Regexp
	blacklistKeywords   []*regexp.Regexp
	whitelistProcesses  []*regexp.Regexp
	whitelistCommands   []*regexp.Regexp
	whitelistNamespaces []*regexp.Regexp
	whitelistPodNames   []*regexp.Regexp
}

type Processor struct {
	ProcPath string
	rules    compiledRules
	mu       sync.RWMutex
}

func compileRules(patterns []string) []*regexp.Regexp {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.L.WithFields(logrus.Fields{"rule": pattern}).WithError(err).Warn("无效的正则表达式规则，已忽略")
			continue
		}
		regexps = append(regexps, re)
	}
	return regexps
}

func NewProcessor(config *models.Config) *Processor {
	p := &Processor{ProcPath: config.Scanner.ProcPath}
	p.UpdateConfig(config)
	return p
}

func (p *Processor) UpdateConfig(config *models.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rules := config.DetectionRules
	p.rules = compiledRules{
		blacklistProcesses:  compileRules(rules.Blacklist.Processes),
		blacklistKeywords:   compileRules(rules.Blacklist.Keywords),
		whitelistProcesses:  compileRules(rules.Whitelist.Processes),
		whitelistCommands:   compileRules(rules.Whitelist.Commands),
		whitelistNamespaces: compileRules(rules.Whitelist.Namespaces),
		whitelistPodNames:   compileRules(rules.Whitelist.PodNames),
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

func matchAny(text string, regexps []*regexp.Regexp) (bool, string) {
	for _, re := range regexps {
		if re.MatchString(text) {
			return true, re.String()
		}
	}
	return false, ""
}

func (p *Processor) AnalyzeProcess(pid int, podNameCache, namespaceCache map[string]string) (*models.ProcessInfo, error) {
	procDir := filepath.Join(p.ProcPath, strconv.Itoa(pid))
	cmdlineFile := filepath.Join(procDir, "cmdline")
	cmdlineData, err := os.ReadFile(cmdlineFile)
	if err != nil {
		return nil, nil
	}
	cmdline := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return nil, nil
	}
	processName := p.getProcessName(cmdline)

	procLogger := log.L.WithFields(logrus.Fields{"pid": pid, "process_name": processName})
	procLogger.Debug("正在分析进程...")

	isBlacklisted, message := p.isBlacklisted(processName, cmdline)
	if !isBlacklisted {
		procLogger.Debug("未命中黑名单，跳过。")
		return nil, nil
	}
	procLogger.WithField("reason", message).Debug("命中黑名单")

	if p.isProcessWhitelisted(processName, cmdline) {
		procLogger.Info("进程在白名单中，已忽略")
		return nil, nil
	}

	containerID := p.getContainerIDFromPID(pid)
	if containerID == "" {
		procLogger.Debug("无法确定容器ID，跳过")
		return nil, nil
	}

	namespace, okNs := namespaceCache[containerID]
	podName, okPn := podNameCache[containerID]
	if !okNs || !okPn {
		procLogger.WithField("containerID", containerID).Debug("缓存未命中，启动按需查询...")
		var err error
		podName, namespace, err = container.GetContainerInfo(containerID)
		if err != nil {
			procLogger.WithField("containerID", containerID).WithError(err).Warn("按需查询容器信息失败")
			return nil, nil
		}
	}

	if p.isInfraWhitelisted(namespace, podName) {
		procLogger.WithFields(logrus.Fields{"namespace": namespace, "pod": podName}).Info("进程所在的基础设施(ns/pod)在白名单中，已忽略")
		return nil, nil
	}

	if !strings.HasPrefix(namespace, "ns-") {
		procLogger.WithField("namespace", namespace).Debug("命名空间不符合 ns- 前缀，跳过")
		return nil, nil
	}
	return &models.ProcessInfo{
		PID:         pid,
		ProcessName: processName,
		Command:     cmdline,
		Timestamp:   time.Now().Format(time.RFC3339),
		ContainerID: containerID,
		Message:     message,
		PodName:     podName,
		Namespace:   namespace,
	}, nil
}

func (p *Processor) isBlacklisted(processName, cmdline string) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if matched, rule := matchAny(processName, p.rules.blacklistProcesses); matched {
		return true, fmt.Sprintf("进程名 '%s' 命中黑名单规则 '%s'", processName, rule)
	}
	if matched, rule := matchAny(cmdline, p.rules.blacklistKeywords); matched {
		return true, fmt.Sprintf("命令行命中关键词黑名单规则 '%s'", rule)
	}
	return false, ""
}

func (p *Processor) isProcessWhitelisted(processName, cmdline string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return matchAnyBool(processName, p.rules.whitelistProcesses) || matchAnyBool(cmdline, p.rules.whitelistCommands)
}

func (p *Processor) isInfraWhitelisted(namespace, podName string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return matchAnyBool(namespace, p.rules.whitelistNamespaces) || matchAnyBool(podName, p.rules.whitelistPodNames)
}

func matchAnyBool(text string, regexps []*regexp.Regexp) bool {
	for _, re := range regexps {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

func (p *Processor) getProcessName(cmdline string) string {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return ""
	}
	return filepath.Base(parts[0])
}

func (p *Processor) getContainerIDFromPID(pid int) string {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	content, err := os.ReadFile(cgroupPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "containerd") || strings.Contains(line, "docker") || strings.Contains(line, "kubepods") {
			parts := strings.Split(line, "/")
			for _, part := range parts {
				if strings.HasPrefix(part, "cri-containerd-") && strings.HasSuffix(part, ".scope") {
					containerID := strings.TrimPrefix(part, "cri-containerd-")
					containerID = strings.TrimSuffix(containerID, ".scope")
					if len(containerID) == 64 && isHexString(containerID) {
						return containerID
					}
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
