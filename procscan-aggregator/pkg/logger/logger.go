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

// Package logger provides logging functionality for procscan-aggregator
package logger

import (
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Entry to provide logging functionality
type Logger struct {
	entry *logrus.Entry
}

var (
	// L is the global logger instance
	L *Logger
)

func init() {
	// Initialize with default logger
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)
	L = &Logger{
		entry: logrus.NewEntry(log),
	}
}

// InitLogger initializes the logger with the specified level and format
func InitLogger(level, format string) {
	log := logrus.New()

	// Set format
	if format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Set level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)

	L = &Logger{
		entry: logrus.NewEntry(log),
	}
}

// WithField adds a single field to the log entry
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		entry: l.entry.WithField(key, value),
	}
}

// WithFields adds multiple fields to the log entry
func (l *Logger) WithFields(fields logrus.Fields) *Logger {
	return &Logger{
		entry: l.entry.WithFields(fields),
	}
}

// WithError adds an error field to the log entry
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		entry: l.entry.WithError(err),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.entry.Debug(msg)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.entry.Info(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.entry.Warn(msg)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.entry.Error(msg)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.entry.Fatal(msg)
}
