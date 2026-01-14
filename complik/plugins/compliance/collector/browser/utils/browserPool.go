// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package utils provides a browser pool implementation for managing headless browser instances.
// The pool supports concurrent access, automatic instance expiration, and graceful cleanup.
// It includes a wait queue mechanism to handle requests when the pool is at capacity.
package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/bearslyricattack/CompliK/complik/pkg/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type BrowserInstance struct {
	Browser  *rod.Browser
	Launcher *launcher.Launcher
	Created  time.Time
	InUse    bool
	PID      int // Chrome process ID for tracking
}

type BrowserPool struct {
	instances   []*BrowserInstance
	mu          sync.RWMutex // Read-write lock for optimized concurrency
	maxSize     int
	maxAge      time.Duration
	waitQueue   chan chan *BrowserInstance // Wait queue for requests when pool is full
	closed      bool
	log         logger.Logger
	cleanupWg   sync.WaitGroup // Wait group for tracking cleanup goroutines
	cleanupDone chan struct{}  // Signal channel for background cleanup goroutine
}

func NewBrowserPool(maxSize int, maxAge time.Duration) *BrowserPool {
	pool := &BrowserPool{
		instances:   make([]*BrowserInstance, 0, maxSize),
		maxSize:     maxSize,
		maxAge:      maxAge,
		waitQueue:   make(chan chan *BrowserInstance, 100), // Buffered queue
		log:         logger.GetLogger().WithField("component", "browser_pool"),
		cleanupDone: make(chan struct{}),
	}

	pool.log.Info("Browser pool created", logger.Fields{
		"max_size":        maxSize,
		"max_age_minutes": maxAge.Minutes(),
	})

	// Start background cleanup goroutine
	go pool.backgroundCleanup()
	return pool
}

func (p *BrowserPool) Get(ctx context.Context) (*BrowserInstance, error) {
	p.mu.RLock()
	for _, instance := range p.instances {
		if !instance.InUse && time.Since(instance.Created) < p.maxAge {
			p.mu.RUnlock()
			p.mu.Lock()
			if !instance.InUse {
				instance.InUse = true
				p.mu.Unlock()
				return instance, nil
			}
			p.mu.Unlock()
			p.mu.RLock()
		}
	}
	p.mu.RUnlock()
	p.mu.Lock()
	if len(p.instances) < p.maxSize {
		instance, err := p.createInstance()
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		instance.InUse = true
		p.instances = append(p.instances, instance)
		p.mu.Unlock()
		return instance, nil
	}
	p.mu.Unlock()
	p.log.Debug("Browser pool full, waiting for available instance")
	waitChan := make(chan *BrowserInstance, 1)
	select {
	case p.waitQueue <- waitChan:
		select {
		case instance := <-waitChan:
			p.log.Debug("Got instance from wait queue")
			return instance, nil
		case <-ctx.Done():
			p.log.Warn("Timeout waiting for browser instance")
			return nil, errors.New("timeout waiting for browser instance")
		}
	default:
		p.log.Error("Browser pool full and wait queue full")
		return nil, errors.New("browser pool is full, cannot create new instance")
	}
}

func (p *BrowserPool) Put(instance *BrowserInstance) {
	if instance == nil {
		p.log.Warn("Attempted to put nil instance back to pool")
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(instance.Created) >= p.maxAge {
		p.log.Info("Instance expired, cleaning up", logger.Fields{
			"pid": instance.PID,
			"age": time.Since(instance.Created).String(),
		})
		p.removeInstance(instance)
		p.cleanupWg.Add(1)
		go func() {
			defer p.cleanupWg.Done()
			p.cleanupInstance(instance)
		}()
		return
	}

	// Check if there are any waiters
	select {
	case waitChan := <-p.waitQueue:
		// Assign directly to waiter
		instance.InUse = true
		p.log.Debug("Reassigning instance to waiter", logger.Fields{
			"pid": instance.PID,
		})
		waitChan <- instance
		return
	default:
		// No waiters, mark as available
		instance.InUse = false
		p.log.Debug("Instance marked as available", logger.Fields{
			"pid": instance.PID,
		})
	}
}

func (p *BrowserPool) createInstance() (*BrowserInstance, error) {
	p.log.Debug("Creating new browser instance")

	l := launcher.New().
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("disable-gpu", "").
		Set("disable-web-security", "").
		Set("disable-features", "VizDisplayCompositor").
		Headless(true)
	u, err := l.Launch()
	if err != nil {
		p.log.Error("Failed to launch browser", logger.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().
		ControlURL(u).
		MustConnect().
		MustIgnoreCertErrors(true)

	// Get PID for tracking
	pid := l.PID()

	instance := &BrowserInstance{
		Browser:  browser,
		Launcher: l,
		Created:  time.Now(),
		InUse:    false,
		PID:      pid,
	}

	p.log.Info("Browser instance created successfully", logger.Fields{
		"instance_count": len(p.instances) + 1,
		"pid":            pid,
		"control_url":    u,
	})

	return instance, nil
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

	if len(expiredInstances) > 0 {
		p.log.Info("Cleaning up expired browser instances", logger.Fields{
			"expired_count":   len(expiredInstances),
			"remaining_count": len(validInstances),
		})
		for _, inst := range expiredInstances {
			p.log.Debug("Expiring instance", logger.Fields{
				"pid": inst.PID,
				"age": time.Since(inst.Created).String(),
			})
		}
	}

	p.instances = validInstances
	for _, instance := range expiredInstances {
		p.cleanupWg.Add(1)
		go func(inst *BrowserInstance) {
			defer p.cleanupWg.Done()
			p.cleanupInstance(inst)
		}(instance)
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
			p.log.Error("Panic during browser cleanup", logger.Fields{
				"panic": r,
				"pid":   instance.PID,
			})
		}
	}()

	pid := instance.PID
	p.log.Info("Starting browser instance cleanup", logger.Fields{
		"pid": pid,
	})

	// Step 1: Try graceful browser close
	if instance.Browser != nil {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		closeDone := make(chan error, 1)
		go func() {
			closeDone <- instance.Browser.Close()
		}()

		select {
		case err := <-closeDone:
			if err != nil {
				p.log.Warn("Browser close failed, will force kill", logger.Fields{
					"error": err.Error(),
					"pid":   pid,
				})
			} else {
				p.log.Info("Browser closed gracefully", logger.Fields{
					"pid": pid,
				})
			}
		case <-closeCtx.Done():
			p.log.Warn("Browser close timeout, forcing kill", logger.Fields{
				"pid": pid,
			})
		}
		closeCancel()
	}

	// Step 2: Kill launcher (sends SIGTERM)
	if instance.Launcher != nil {
		p.log.Debug("Killing launcher with SIGTERM", logger.Fields{
			"pid": pid,
		})
		instance.Launcher.Kill()
		time.Sleep(500 * time.Millisecond)

		// Step 3: Verify process is actually dead
		if p.isProcessAlive(pid) {
			p.log.Warn("Process still alive after SIGTERM, sending SIGKILL", logger.Fields{
				"pid": pid,
			})
			p.forceKillProcess(pid)
			time.Sleep(300 * time.Millisecond)

			// Final verification
			if p.isProcessAlive(pid) {
				p.log.Error("ZOMBIE PROCESS DETECTED: Process still alive after SIGKILL", logger.Fields{
					"pid": pid,
				})
			} else {
				p.log.Info("Process successfully killed with SIGKILL", logger.Fields{
					"pid": pid,
				})
			}
		} else {
			p.log.Info("Process terminated successfully", logger.Fields{
				"pid": pid,
			})
		}
	}
}

// isProcessAlive checks if a process with the given PID is still running
func (p *BrowserPool) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Send signal 0 to check if process exists without actually sending a signal
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// forceKillProcess sends SIGKILL to forcefully terminate a process
func (p *BrowserPool) forceKillProcess(pid int) {
	if pid <= 0 {
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		p.log.Warn("Failed to find process for force kill", logger.Fields{
			"pid":   pid,
			"error": err.Error(),
		})
		return
	}
	if err := process.Signal(syscall.SIGKILL); err != nil {
		p.log.Error("Failed to send SIGKILL", logger.Fields{
			"pid":   pid,
			"error": err.Error(),
		})
	}
}

func (p *BrowserPool) backgroundCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	p.log.Info("Background cleanup goroutine started")

	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				p.log.Info("Background cleanup goroutine stopped (pool closed)")
				return
			}
			p.log.Debug("Running periodic cleanup check")
			p.cleanupExpired()
			p.mu.Unlock()
		case <-p.cleanupDone:
			p.log.Info("Background cleanup goroutine received shutdown signal")
			return
		}
	}
}

// Close closes the browser pool and cleans up all instances.
func (p *BrowserPool) Close() {
	p.log.Info("Closing browser pool")

	p.mu.Lock()
	p.closed = true
	instances := p.instances
	p.instances = nil
	p.mu.Unlock()

	// Signal background cleanup to stop
	close(p.cleanupDone)

	p.log.Info("Cleaning up browser instances", logger.Fields{
		"instance_count": len(instances),
	})

	// Clean up all instances and wait for completion
	for _, instance := range instances {
		p.cleanupWg.Add(1)
		go func(inst *BrowserInstance) {
			defer p.cleanupWg.Done()
			p.cleanupInstance(inst)
		}(instance)
	}

	// Wait for all cleanup goroutines to finish with timeout
	cleanupDone := make(chan struct{})
	go func() {
		p.cleanupWg.Wait()
		close(cleanupDone)
	}()

	select {
	case <-cleanupDone:
		p.log.Info("All browser instances cleaned up successfully")
	case <-time.After(30 * time.Second):
		p.log.Error("Timeout waiting for browser cleanup to complete")
	}

	// Close wait queue
	close(p.waitQueue)

	p.log.Info("Browser pool closed")
}
