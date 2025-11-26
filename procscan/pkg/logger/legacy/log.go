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

// Package log provides a legacy global logger instance for backward compatibility.
// New code should prefer using the logger package instead.
package log

import (
	"github.com/sirupsen/logrus"
	"os"
)

// L is a global, standardized logrus logger instance.
var L = logrus.New()

func init() {
	// Set log output format to JSON.
	L.SetFormatter(&logrus.JSONFormatter{})
	// Set log output to standard output.
	L.SetOutput(os.Stdout)
	// Set an initial default level.
	L.SetLevel(logrus.InfoLevel)
}

// SetLevel parses and sets the global logger level from a string.
func SetLevel(levelStr string) {
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		L.WithField("error", err).Warnf("Invalid log level '%s', will continue using current level", levelStr)
		return
	}
	L.SetLevel(level)
	L.WithField("new_level", level.String()).Info("Log level updated")
}
