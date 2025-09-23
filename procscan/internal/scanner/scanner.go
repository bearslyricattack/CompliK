package scanner

import (
	"context"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"log"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/internal/alert"
	"github.com/bearslyricattack/CompliK/procscan/internal/container"
	"github.com/bearslyricattack/CompliK/procscan/internal/process"
)

// Scanner 扫描器结构
type Scanner struct {
	config        *models.Config
	processor     *process.Processor
	containerInfo *container.InfoProvider
	alertSender   *alert.Sender
	scanInterval  time.Duration
}

// NewScanner 创建新的扫描器
func NewScanner(config *models.Config) *Scanner {
	return &Scanner{
		config:        config,
		processor:     nil, // 在Start方法中初始化
		containerInfo: container.NewInfoProvider(),
		alertSender:   alert.NewSender(),
		scanInterval:  time.Duration(config.ScanIntervalSecond) * time.Second,
	}
}

// Start 启动扫描器
func (s *Scanner) Start(ctx context.Context) error {
	log.Printf("启动进程扫描器，节点: %s, 扫描间隔: %v",
		s.config.NodeName, s.scanInterval)

	// 创建进程处理器
	s.processor = process.NewProcessor(
		s.config,
	)

	// 启动定时任务
	return s.runScanLoop(ctx)
}

func (s *Scanner) UpdateConfig(config *models.Config) {
	s.config = config
	s.scanInterval = time.Duration(config.ScanIntervalSecond) * time.Second
	if s.processor != nil {
		s.processor.UpdateConfig(config)
	}
}

// runScanLoop 运行扫描循环
func (s *Scanner) runScanLoop(ctx context.Context) error {
	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("扫描器停止")
			return ctx.Err()

		case <-ticker.C:
			if err := s.scanProcesses(); err != nil {
				log.Printf("扫描进程失败: %v", err)
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

	maliciousProcesses := make([]models.ProcessInfo, 0)

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

	if len(maliciousProcesses) > 0 {
		return s.alertSender.SendBatchAlerts(maliciousProcesses)
	}

	return nil
}
