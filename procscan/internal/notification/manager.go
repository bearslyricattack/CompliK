package notification

import (
	"fmt"
	"strings"
	"sync"
)

// Manager manages multiple notification channels
type Manager struct {
	notifiers []notifier
	mu        sync.RWMutex
}

// notifier defines the interface for notification channels
type notifier interface {
	Send(message string) error
}

// ThreatNotifier defines the interface for threat-specific notifications
type ThreatNotifier interface {
	SendThreatAlert(threat ThreatInfo) error
}

// ThreatInfo represents threat information structure
type ThreatInfo struct {
	TotalAffected int            `json:"total_affected"`
	ScanTime      string         `json:"scan_time"`
	NodeName      string         `json:"node_name"`
	Threats       []ThreatDetail `json:"threats"`
	Actions       []string       `json:"actions"`
}

// ThreatDetail contains detailed threat information for a namespace
type ThreatDetail struct {
	Namespace    string        `json:"namespace"`
	ProcessCount int           `json:"process_count"`
	Processes    []ProcessInfo `json:"processes"`
	ActionResult string        `json:"action_result"`
}

// ProcessInfo contains enhanced process information
type ProcessInfo struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	Command string `json:"command"`
	User    string `json:"user"`
	Status  string `json:"status"`
	// Kubernetes related information
	PodName        string `json:"pod_name"`
	PodNamespace   string `json:"pod_namespace"`
	PodUID         string `json:"pod_uid"`
	ContainerName  string `json:"container_name"`
	ContainerID    string `json:"container_id"`
	ContainerImage string `json:"container_image"`
	NodeName       string `json:"node_name"`
	PodIP          string `json:"pod_ip"`
	// Container runtime information
	Runtime        string `json:"runtime"`
	ContainerState string `json:"container_state"`
	// Security related information
	SecurityContext map[string]interface{} `json:"security_context"`
}

// NewManager creates a new notification manager
func NewManager() *Manager {
	return &Manager{
		notifiers: []notifier{},
	}
}

// AddNotifier adds a notifier to the manager
func (m *Manager) AddNotifier(n notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notifiers = append(m.notifiers, n)
}

// SendNotifications sends a message to all registered notifiers
func (m *Manager) SendNotifications(message string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if message == "" {
		return nil
	}

	var errors []string
	for _, notifier := range m.notifiers {
		if err := notifier.Send(message); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some notifications failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// SendThreatAlert sends a threat alert to all threat-capable notifiers
func (m *Manager) SendThreatAlert(threat ThreatInfo) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errors []string
	for _, notifier := range m.notifiers {
		if threatNotifier, ok := notifier.(ThreatNotifier); ok {
			if err := threatNotifier.SendThreatAlert(threat); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some threat notifications failed: %s", strings.Join(errors, "; "))
	}

	return nil
}
