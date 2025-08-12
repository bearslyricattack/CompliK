package website

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"log"
	"sync"
	"time"
)

type BrowserInstance struct {
	Browser  *rod.Browser
	Launcher *launcher.Launcher
	Created  time.Time
	InUse    bool
}

type BrowserPool struct {
	instances []*BrowserInstance
	mu        sync.Mutex
	maxSize   int
	maxAge    time.Duration
}

// NewBrowserPool 创建新的浏览器池
func NewBrowserPool(maxSize int, maxAge time.Duration) *BrowserPool {
	return &BrowserPool{
		instances: make([]*BrowserInstance, 0, maxSize),
		maxSize:   maxSize,
		maxAge:    maxAge,
	}
}

// Get 获取浏览器实例
func (p *BrowserPool) Get() (*BrowserInstance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 查找可用的实例
	for _, instance := range p.instances {
		if !instance.InUse && time.Since(instance.Created) < p.maxAge {
			instance.InUse = true
			return instance, nil
		}
	}

	// 没有可用实例，创建新的
	if len(p.instances) < p.maxSize {
		instance, err := p.createInstance()
		if err != nil {
			return nil, err
		}
		instance.InUse = true
		p.instances = append(p.instances, instance)
		return instance, nil
	}

	// 池满了，清理过期实例后重试
	p.cleanupExpired()

	// 再次尝试创建
	if len(p.instances) < p.maxSize {
		instance, err := p.createInstance()
		if err != nil {
			return nil, err
		}
		instance.InUse = true
		p.instances = append(p.instances, instance)
		return instance, nil
	}

	return nil, fmt.Errorf("浏览器池已满，无法创建新实例")
}

// Put 归还浏览器实例
func (p *BrowserPool) Put(instance *BrowserInstance) {
	if instance == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查实例是否过期
	if time.Since(instance.Created) >= p.maxAge {
		p.removeInstance(instance)
		go p.cleanupInstance(instance)
		return
	}

	// 标记为未使用
	instance.InUse = false
}

// 创建新的浏览器实例
func (p *BrowserPool) createInstance() (*BrowserInstance, error) {
	l := launcher.New().
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("disable-gpu", "").
		Set("disable-web-security", "").
		Set("disable-features", "VizDisplayCompositor").
		Headless(true)

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %v", err)
	}

	browser := rod.New().
		ControlURL(u).
		MustConnect().
		MustIgnoreCertErrors(true)

	return &BrowserInstance{
		Browser:  browser,
		Launcher: l,
		Created:  time.Now(),
		InUse:    false,
	}, nil
}

// 清理过期实例（需要在锁内调用）
func (p *BrowserPool) cleanupExpired() {
	var validInstances []*BrowserInstance
	var expiredInstances []*BrowserInstance

	for _, instance := range p.instances {
		if time.Since(instance.Created) >= p.maxAge || instance.Browser == nil {
			expiredInstances = append(expiredInstances, instance)
		} else {
			validInstances = append(validInstances, instance)
		}
	}

	p.instances = validInstances

	// 异步清理过期实例
	for _, instance := range expiredInstances {
		go p.cleanupInstance(instance)
	}
}

// 从池中移除实例（需要在锁内调用）
func (p *BrowserPool) removeInstance(target *BrowserInstance) {
	for i, instance := range p.instances {
		if instance == target {
			p.instances = append(p.instances[:i], p.instances[i+1:]...)
			break
		}
	}
}

// 异步清理单个实例
func (p *BrowserPool) cleanupInstance(instance *BrowserInstance) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("清理浏览器实例时发生panic: %v", r)
		}
	}()

	if instance.Browser != nil {
		err := instance.Browser.Close()
		if err != nil {
			return
		}
	}
	if instance.Launcher != nil {
		instance.Launcher.Kill()
	}
}
