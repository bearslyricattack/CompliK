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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Scanner status metrics
	ScannerRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "procscan_scanner_running",
		Help: "Indicates whether the scanner is currently running (1 for running, 0 for stopped)",
	})

	ScannerUptimeSeconds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_scanner_uptime_seconds",
		Help: "Total uptime of the scanner in seconds",
	})

	ScanDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "procscan_scan_duration_seconds",
		Help:    "Time taken to complete a single scan",
		Buckets: prometheus.DefBuckets,
	})

	ScanTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_scan_total",
		Help: "Total number of scans performed",
	})

	ScanErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_scan_errors_total",
		Help: "Total number of scan errors",
	})

	// Threat detection metrics
	ThreatsDetectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_threats_detected_total",
		Help: "Total number of threats detected",
	})

	ThreatsByType = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "procscan_threats_by_type",
		Help: "Number of threats detected by type",
	}, []string{"threat_type"})

	ThreatsBySeverity = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "procscan_threats_by_severity",
		Help: "Number of threats detected by severity level",
	}, []string{"severity"})

	SuspiciousProcessesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_suspicious_processes_total",
		Help: "Total number of suspicious processes detected",
	})

	SuspiciousProcessesByNamespace = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "procscan_suspicious_processes_by_namespace",
		Help: "Number of suspicious processes detected by namespace",
	}, []string{"namespace"})

	// Response action metrics
	LabelActionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_label_actions_total",
		Help: "Total number of label actions attempted",
	})

	LabelActionsSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_label_actions_success_total",
		Help: "Total number of successful label actions",
	})

	NotificationsSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_notifications_sent_total",
		Help: "Total number of notifications sent",
	})

	NotificationsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_notifications_failed_total",
		Help: "Total number of failed notification attempts",
	})

	// Performance metrics
	ProcessesAnalyzedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "procscan_processes_analyzed_total",
		Help: "Total number of processes analyzed",
	})

	MemoryUsageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "procscan_memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})

	CPUUsagePercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "procscan_cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})
)
