package utils

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

func NewBrowserPool(maxSize int, maxAge time.Duration) *BrowserPool {
	return &BrowserPool{
		instances: make([]*BrowserInstance, 0, maxSize),
		maxSize:   maxSize,
		maxAge:    maxAge,
	}
}

func (p *BrowserPool) Get() (*BrowserInstance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, instance := range p.instances {
		if !instance.InUse && time.Since(instance.Created) < p.maxAge {
			instance.InUse = true
			return instance, nil
		}
	}

	if len(p.instances) < p.maxSize {
		instance, err := p.createInstance()
		if err != nil {
			return nil, err
		}
		instance.InUse = true
		p.instances = append(p.instances, instance)
		return instance, nil
	}

	p.cleanupExpired()
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

func (p *BrowserPool) Put(instance *BrowserInstance) {
	if instance == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(instance.Created) >= p.maxAge {
		p.removeInstance(instance)
		go p.cleanupInstance(instance)
		return
	}
	instance.InUse = false
}

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
	for _, instance := range expiredInstances {
		go p.cleanupInstance(instance)
	}
}

func (p *BrowserPool) removeInstance(target *BrowserInstance) {
	for i, instance := range p.instances {
		if instance == target {
			p.instances = append(p.instances[:i], p.instances[i+1:]...)
			break
		}
	}
}

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
