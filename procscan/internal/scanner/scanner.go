package scanner

import (
	"context"
	"fmt"
	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/internal/alert"
	"github.com/bearslyricattack/CompliK/procscan/internal/container"
	"github.com/bearslyricattack/CompliK/procscan/internal/k8s"
	"github.com/bearslyricattack/CompliK/procscan/internal/process"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"k8s.io/client-go/kubernetes"
)

type Scanner struct {
	config    *models.Config
	processor *process.Processor
	k8sClient *kubernetes.Clientset
	mu        sync.RWMutex
	ticker    *time.Ticker
}

func NewScanner(config *models.Config) *Scanner {
	return &Scanner{config: config}
}

func (s *Scanner) UpdateConfig(newConfig *models.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.L.Info("开始应用新配置...")
	oldConfig := s.config
	s.config = newConfig

	if oldConfig.Scanner.LogLevel != newConfig.Scanner.LogLevel {
		log.SetLevel(newConfig.Scanner.LogLevel)
	}
	if oldConfig.Actions.Annotation.Enabled != newConfig.Actions.Annotation.Enabled {
		log.L.WithFields(logrus.Fields{"key": "actions.annotation.enabled", "from": oldConfig.Actions.Annotation.Enabled, "to": newConfig.Actions.Annotation.Enabled}).Info("配置项变更")
	}
	if oldConfig.Actions.ForceDelete.Enabled != newConfig.Actions.ForceDelete.Enabled {
		log.L.WithFields(logrus.Fields{"key": "actions.forceDelete.enabled", "from": oldConfig.Actions.ForceDelete.Enabled, "to": newConfig.Actions.ForceDelete.Enabled}).Info("配置项变更")
	}

	oldInterval := oldConfig.Scanner.ScanInterval
	newInterval := newConfig.Scanner.ScanInterval
	if oldInterval != newInterval {
		if s.ticker != nil {
			s.ticker.Reset(newInterval)
		}
		log.L.WithFields(logrus.Fields{"key": "scanner.scan_interval", "from": oldInterval.String(), "to": newInterval.String()}).Info("配置项变更")
	}

	s.processor.UpdateConfig(newConfig)
	log.L.Info("检测规则已刷新。")

	log.L.Info("配置已成功热加载。")
}

func (s *Scanner) Start(ctx context.Context) error {
	s.processor = process.NewProcessor(s.config)

	k8sClient, err := k8s.NewK8sClient()
	if err != nil {
		log.L.WithError(err).Warn("初始化k8s客户端失败，与k8s相关的操作将被跳过")
	} else {
		s.k8sClient = k8sClient
		log.L.Info("k8s客户端初始化成功。")
	}

	initialInterval := s.config.Scanner.ScanInterval
	s.ticker = time.NewTicker(initialInterval)

	log.L.WithFields(logrus.Fields{"node": os.Getenv("NODE_NAME"), "interval": initialInterval.String()}).Info("进程扫描器启动")
	return s.runScanLoop(ctx)
}

func (s *Scanner) runScanLoop(ctx context.Context) error {
	defer s.ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.L.Info("扫描器停止")
			return ctx.Err()
		case <-s.ticker.C:
			if err := s.scanProcesses(); err != nil {
				log.L.WithError(err).Error("扫描进程失败")
			}
		}
	}
}

func (s *Scanner) scanProcesses() error {
	log.L.Info("开始新一轮扫描...")

	s.mu.RLock()
	currentConfig := s.config
	s.mu.RUnlock()

	log.L.Debug("正在构建容器信息缓存...")
	podNameCache, namespaceCache, err := container.BuildContainerCache()
	if err != nil {
		return fmt.Errorf("构建容器缓存失败: %w", err)
	}
	log.L.WithField("count", len(podNameCache)).Debug("容器信息缓存构建完成")

	pids, err := s.processor.GetAllProcesses()
	if err != nil {
		return err
	}
	log.L.WithField("count", len(pids)).Info("开始分析进程...")

	numWorkers := runtime.NumCPU()
	pidChan := make(chan int, len(pids))
	resultsChan := make(chan *models.ProcessInfo, len(pids))
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for pid := range pidChan {
				processInfo, _ := s.processor.AnalyzeProcess(pid, podNameCache, namespaceCache)
				if processInfo != nil {
					resultsChan <- processInfo
				}
			}
		}(i)
	}

	for _, pid := range pids {
		pidChan <- pid
	}
	close(pidChan)

	wg.Wait()
	close(resultsChan)
	log.L.Info("所有进程分析完成。")

	resultsByNamespace := make(map[string][]*models.ProcessInfo)
	for processInfo := range resultsChan {
		resultsByNamespace[processInfo.Namespace] = append(resultsByNamespace[processInfo.Namespace], processInfo)
	}

	if len(resultsByNamespace) == 0 {
		log.L.Info("本轮扫描未发现符合条件的可疑进程。")
		return nil
	}

	log.L.WithField("count", len(resultsByNamespace)).Info("发现存在可疑进程的命名空间，开始分组处理...")
	finalResults := make([]*alert.NamespaceScanResult, 0, len(resultsByNamespace))
	for namespace, processInfos := range resultsByNamespace {
		annotationResult, deletionResult := s.handleGroupedActions(namespace, currentConfig)
		finalResults = append(finalResults, &alert.NamespaceScanResult{
			Namespace:        namespace,
			ProcessInfos:     processInfos,
			AnnotationResult: annotationResult,
			DeletionResult:   deletionResult,
		})
	}

	if err := alert.SendGlobalBatchAlert(finalResults, currentConfig.Notifications.Lark.Webhook); err != nil {
		log.L.WithError(err).Error("发送全局批量飞书告警失败")
	}

	log.L.Info("本轮扫描结束。")
	return nil
}

func (s *Scanner) handleGroupedActions(namespace string, config *models.Config) (annotationResult string, deletionResult string) {
	if config.Actions.Annotation.Enabled {
		if s.k8sClient != nil {
			annotations := config.Actions.Annotation.Data
			if len(annotations) == 0 {
				annotations = map[string]string{"debt.sealos/status": "Suspend"}
			}
			log.L.WithFields(logrus.Fields{"namespace": namespace, "annotations": annotations}).Info("开始为命名空间添加注解")
			if err := k8s.AnnotateNamespace(s.k8sClient, namespace, annotations); err != nil {
				log.L.WithFields(logrus.Fields{"namespace": namespace}).WithError(err).Error("为命名空间添加注解失败")
				annotationResult = fmt.Sprintf("失败: %v", err)
			} else {
				annotationResult = "成功"
				if config.Actions.ForceDelete.Enabled {
					if err := k8s.ForceDeleteAbnormalPodsInNamespace(s.k8sClient, namespace); err != nil {
						log.L.WithFields(logrus.Fields{"namespace": namespace}).WithError(err).Error("强制删除 Pods 失败")
						deletionResult = fmt.Sprintf("失败: %v", err)
					} else {
						deletionResult = "成功"
					}
				}
			}
		} else {
			annotationResult = "无法执行 (K8s客户端不可用)"
		}
	} else {
		annotationResult = "功能已禁用"
	}
	return
}