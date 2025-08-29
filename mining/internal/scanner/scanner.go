package scanner

import (
	"context"
	"github.com/bearslyricattack/CompliK/mining/internal/alert"
	"github.com/bearslyricattack/CompliK/mining/internal/config"
	"github.com/bearslyricattack/CompliK/mining/internal/container"
	"github.com/bearslyricattack/CompliK/mining/internal/process"
	"github.com/bearslyricattack/CompliK/mining/internal/types"
	"log"
	"time"
)

// Scanner 扫描器结构
type Scanner struct {
	config        types.ScannerConfig
	configManager *config.Manager
	processor     *process.Processor
	containerInfo *container.InfoProvider
	alertSender   *alert.Sender
	scanInterval  time.Duration
}

// NewScanner 创建新的扫描器
func NewScanner() *Scanner {
	// 加载扫描器配置
	scannerConfig := config.LoadScannerConfig()
	// 创建配置管理器
	configManager := config.NewManager(scannerConfig.ConfigMapPath)
	// 创建告警发送器
	alertSender := alert.NewSender(scannerConfig.ComplianceURL, scannerConfig.NodeName)
	// 创建容器信息提供者
	containerInfoProvider := container.NewInfoProvider(scannerConfig.ProcPath)
	return &Scanner{
		config:        scannerConfig,
		configManager: configManager,
		containerInfo: containerInfoProvider,
		alertSender:   alertSender,
		scanInterval:  time.Duration(scannerConfig.ScanInterval) * time.Second,
	}
}

// Start 启动扫描器
func (s *Scanner) Start(ctx context.Context) error {
	log.Printf("启动进程扫描器，节点: %s, 扫描间隔: %v",
		s.config.NodeName, s.scanInterval)

	// 加载初始配置
	if err := s.configManager.LoadConfig(); err != nil {
		return err
	}

	// 创建进程处理器
	s.processor = process.NewProcessor(
		s.config.ProcPath,
		s.config.NodeName,
		s.configManager.GetConfig(),
	)

	// 启动定时任务
	return s.runScanLoop(ctx)
}

// runScanLoop 运行扫描循环
func (s *Scanner) runScanLoop(ctx context.Context) error {
	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	// 配置重新加载定时器（每5分钟重新加载一次配置）
	configTicker := time.NewTicker(5 * time.Minute)
	defer configTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("扫描器停止")
			return ctx.Err()

		case <-ticker.C:
			if err := s.scanProcesses(); err != nil {
				log.Printf("扫描进程失败: %v", err)
			}

		case <-configTicker.C:
			if err := s.reloadConfig(); err != nil {
				log.Printf("重新加载配置失败: %v", err)
			}
		}
	}
}

// scanProcesses 扫描进程
func (s *Scanner) scanProcesses() error {
	pids, err := s.processor.GetAllProcesses()
	if err != nil {
		return err
	}
	var maliciousProcesses []types.ProcessInfo
	for _, pid := range pids {
		processInfo, err := s.processor.AnalyzeProcess(pid)
		if err != nil {
			log.Printf("检查进程 %d 失败: %v", pid, err)
			continue
		}
		if processInfo == nil {
			continue
		}

		log.Printf("发现恶意进程: PID=%d, 进程名=%s, 命令行=%s",
			processInfo.PID, processInfo.ProcessName, processInfo.Command)

		// 获取容器信息
		containerInfo, err := s.containerInfo.GetContainerInfo(pid)
		if err != nil {
			log.Printf("获取容器信息失败: %v", err)
		} else {
			processInfo.ContainerID = containerInfo.ContainerID
			processInfo.PodName = containerInfo.PodName
			processInfo.Namespace = containerInfo.Namespace
		}

		maliciousProcesses = append(maliciousProcesses, *processInfo)
	}

	// 批量发送告警
	if len(maliciousProcesses) > 0 {
		return s.alertSender.SendBatchAlerts(maliciousProcesses)
	}

	return nil
}

func (s *Scanner) reloadConfig() error {
	if err := s.configManager.LoadConfig(); err != nil {
		return err
	}
	s.processor.UpdateConfig(s.configManager.GetConfig())
	return nil
}
