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

// Package api provides HTTP API handlers for the scanner service
package api

import (
	"encoding/json"
	"net/http"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"github.com/sirupsen/logrus"
)

// ViolationRecordsProvider 定义获取违规记录的接口
type ViolationRecordsProvider interface {
	GetViolationRecords() []*models.ViolationRecord
}

// Handler API 处理器
type Handler struct {
	provider ViolationRecordsProvider
}

// NewHandler 创建新的 API 处理器
func NewHandler(provider ViolationRecordsProvider) *Handler {
	return &Handler{
		provider: provider,
	}
}

// GetViolationsHandler 返回当前所有违规记录
func (h *Handler) GetViolationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	records := h.provider.GetViolationRecords()

	legacy.L.WithFields(logrus.Fields{
		"count":  len(records),
		"remote": r.RemoteAddr,
	}).Info("API: Returning violation records")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		legacy.L.WithError(err).Error("Failed to encode violation records")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// HealthHandler 健康检查接口
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
