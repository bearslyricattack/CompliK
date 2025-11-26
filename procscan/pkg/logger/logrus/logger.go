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

// Package logrus provides a simplified wrapper around the logrus logging library.
package logrus

import (
	"github.com/sirupsen/logrus"
)

// Logger is a simplified logger wrapper
type Logger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
}

// NewLogger creates a new logrus logger instance
func NewLogger() *Logger {
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	return &Logger{
		logger: l,
		entry:  l.WithFields(logrus.Fields{}),
	}
}

// Debug outputs debug level log
func (l *Logger) Debug(msg string) {
	l.entry.Debug(msg)
}

// Info outputs info level log
func (l *Logger) Info(msg string) {
	l.entry.Info(msg)
}

// Warn outputs warning level log
func (l *Logger) Warn(msg string) {
	l.entry.Warn(msg)
}

// Error outputs error level log
func (l *Logger) Error(msg string) {
	l.entry.Error(msg)
}

// Fatal outputs fatal error log and exits
func (l *Logger) Fatal(msg string) {
	l.entry.Fatal(msg)
}

// WithField adds a single field to the log entry
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithField(key, value),
	}
}

// WithFields adds multiple fields to the log entry
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithFields(fields),
	}
}

// WithError adds an error field to the log entry
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithError(err),
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level string) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	l.logger.SetLevel(logLevel)
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() string {
	return l.logger.GetLevel().String()
}
