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

// Package logger provides a simplified logging interface for the procscan application.
// It wraps the logrus implementation and provides convenient global functions.
package logger

import (
	"sync"

	"github.com/bearslyricattack/CompliK/procscan/pkg/logger/logrus"
)

var (
	defaultLogger *logrus.Logger
	once          sync.Once
)

// Init initializes the default logger
func Init() {
	once.Do(func() {
		defaultLogger = logrus.NewLogger()
	})
}

// GetLogger returns the default logger instance
func GetLogger() *logrus.Logger {
	if defaultLogger == nil {
		Init()
	}
	return defaultLogger
}

// SetLevel sets the log level
func SetLevel(level string) {
	GetLogger().SetLevel(level)
}

// Debug outputs debug level log
func Debug(msg string) {
	GetLogger().Debug(msg)
}

// Info outputs info level log
func Info(msg string) {
	GetLogger().Info(msg)
}

// Warn outputs warning level log
func Warn(msg string) {
	GetLogger().Warn(msg)
}

// Error outputs error level log
func Error(msg string) {
	GetLogger().Error(msg)
}

// Fatal outputs fatal error log and exits
func Fatal(msg string) {
	GetLogger().Fatal(msg)
}

// WithField adds a single field to the log entry
func WithField(key string, value interface{}) *logrus.Logger {
	return GetLogger().WithField(key, value)
}

// WithFields adds multiple fields to the log entry
func WithFields(fields map[string]interface{}) *logrus.Logger {
	return GetLogger().WithFields(fields)
}

// WithError adds an error field to the log entry
func WithError(err error) *logrus.Logger {
	return GetLogger().WithError(err)
}
