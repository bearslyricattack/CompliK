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

package config

import (
	"context"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// UpdateHandler is called when configuration is updated
type UpdateHandler func(*models.Config)

// Watcher watches configuration file for changes and triggers updates
type Watcher struct {
	loader  *Loader
	watcher *fsnotify.Watcher
	handler UpdateHandler
}

// NewWatcher creates a new configuration file watcher
func NewWatcher(loader *Loader, handler UpdateHandler) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		loader:  loader,
		watcher: fsWatcher,
		handler: handler,
	}, nil
}

// Start begins watching the configuration file for changes
func (w *Watcher) Start(ctx context.Context) error {
	configDir := w.loader.GetConfigDir()
	if err := w.watcher.Add(configDir); err != nil {
		return err
	}

	legacy.L.WithField("path", w.loader.GetConfigPath()).Info("Started monitoring configuration file")

	go w.watchLoop(ctx)
	return nil
}

// Stop stops the configuration watcher
func (w *Watcher) Stop() error {
	return w.watcher.Close()
}

// watchLoop runs the file watching loop
func (w *Watcher) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				w.handleFileChange()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			legacy.L.WithField("error", err).Error("File watcher error")
		}
	}
}

// handleFileChange handles configuration file changes
func (w *Watcher) handleFileChange() {
	changed, err := w.loader.HasChanged()
	if err != nil {
		legacy.L.WithError(err).Error("Failed to check configuration file changes")
		return
	}

	if !changed {
		return
	}

	legacy.L.WithFields(logrus.Fields{
		"file": w.loader.GetConfigPath(),
	}).Info("Detected configuration file content change, preparing hot reload...")

	newConfig, err := w.loader.Load()
	if err != nil {
		legacy.L.WithField("error", err).Error("Failed to load new configuration during hot reload, continuing with old configuration")
		return
	}

	if w.handler != nil {
		w.handler(newConfig)
	}
}
