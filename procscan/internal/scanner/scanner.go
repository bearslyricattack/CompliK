package scanner

import (
	"context"
	"github.com/bearslyricattack/CompliK/procscan/internal/alert"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"log"
	"os"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/internal/container"
	"github.com/bearslyricattack/CompliK/procscan/internal/process"
)

type Scanner struct {
	config       *models.Config
	processor    *process.Processor
	scanInterval time.Duration
}

func NewScanner(config *models.Config) *Scanner {
	return &Scanner{
		config:       config,
		processor:    nil,
		scanInterval: time.Duration(config.ScanIntervalSecond) * time.Second,
	}
}

func (s *Scanner) Start(ctx context.Context) error {
	log.Printf("启动进程扫描器，节点: %s, 扫描间隔: %v", os.Getenv("NODE_NAME"), s.scanInterval)
	s.processor = process.NewProcessor(s.config)
	return s.runScanLoop(ctx)
}

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

func (s *Scanner) scanProcesses() error {
	pids, err := s.processor.GetAllProcesses()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		processInfo, err := s.processor.AnalyzeProcess(pid)
		if err != nil {
			log.Printf("检查进程 %d 失败: %v", pid, err)
			continue
		}
		if processInfo == nil {
			continue
		}
		podName, namespace, err := container.GetContainerInfo(processInfo.ContainerID)
		if err != nil {
			log.Printf("获取容器信息失败: %v", err)
		} else {
			processInfo.PodName = podName
			processInfo.Namespace = namespace
		}
		log.Printf("发现恶意进程: PID=%d, 进程名=%s, 命令行=%s,containerid=%s,PodName=%s,Namespace=%s,原因=%s", processInfo.PID, processInfo.ProcessName, processInfo.Command, processInfo.ContainerID, processInfo.PodName, processInfo.Namespace, processInfo.Message)
		err = alert.SendProcessAlert(processInfo, s.config.Lark)
		if err != nil {
			return err
		}
	}
	return nil
}
