package alert

import (
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"net/http"
	"time"
)

type Sender struct {
	complianceURL string
	httpClient    *http.Client
	nodeName      string
}

func NewSender(complianceURL, nodeName string) *Sender {
	return &Sender{
		complianceURL: complianceURL,
		nodeName:      nodeName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Sender) SendAlert(processInfo models.ProcessInfo) error {
	return nil
}
