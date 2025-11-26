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

package plugin

import (
	"context"

	"github.com/bearslyricattack/CompliK/complik/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/complik/pkg/utils/config"
)

// Plugin represents a pluggable component with lifecycle management.
type Plugin interface {
	Name() string
	Type() string

	// Start initializes the plugin with context, config, and event bus.
	Start(ctx context.Context, pluginConfig config.PluginConfig, eventBus *eventbus.EventBus) error
	Stop(ctx context.Context) error
}

// PluginFactory creates plugin instances by name.
type PluginFactory func(name string) Plugin
