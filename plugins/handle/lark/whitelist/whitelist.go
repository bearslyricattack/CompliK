package whitelist

import (
	"gorm.io/gorm"
	"time"
)

type WhitelistType string

const (
	WhitelistTypeNamespace WhitelistType = "namespace"
	WhitelistTypeHost      WhitelistType = "host"
)

type Whitelist struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Region    string        `json:"region"`
	Name      string        `gorm:"not null;index" json:"name"`
	Namespace string        `gorm:"index" json:"namespace"`
	Hostname  string        `gorm:"index" json:"hostname"`
	Type      WhitelistType `gorm:"not null;index" json:"type"`
	Remark    string        `gorm:"type:text" json:"remark"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func (Whitelist) TableName() string {
	return "whitelists"
}

type WhitelistService struct {
	db *gorm.DB
}

func NewWhitelistService(db *gorm.DB) *WhitelistService {
	return &WhitelistService{db: db}
}

func (s *WhitelistService) IsNamespaceWhitelisted(namespace string) (bool, error) {
	var count int64
	err := s.db.Model(&Whitelist{}).
		Where("namespace = ? AND type = ?", namespace, WhitelistTypeNamespace).
		Count(&count)
	if err != nil {
		return false, err.Error
	}
	return count > 0, nil
}

func (s *WhitelistService) IsHostWhitelisted(host string) (bool, error) {
	var count int64
	err := s.db.Model(&Whitelist{}).
		Where("hostname = ? AND type = ?", host, WhitelistTypeHost).
		Count(&count)
	if err != nil {
		return false, err.Error
	}
	return count > 0, nil
}

// IsWhitelisted 综合检查是否在白名单中（命名空间或主机任一匹配即为白名单）
func (s *WhitelistService) IsWhitelisted(namespace, host string) (bool, error) {
	if namespace != "" {
		isNamespaceWhitelisted, err := s.IsNamespaceWhitelisted(namespace)
		if err != nil {
			return false, err
		}
		if isNamespaceWhitelisted {
			return true, nil
		}
	}

	if host != "" {
		isHostWhitelisted, err := s.IsHostWhitelisted(host)
		if err != nil {
			return false, err
		}
		if isHostWhitelisted {
			return true, nil
		}
	}

	return false, nil
}

func (s *WhitelistService) AddNamespaceWhitelist(name, namespace, remark string) error {
	whitelist := &Whitelist{
		Name:      name,
		Namespace: namespace,
		Type:      WhitelistTypeNamespace,
		Remark:    remark,
	}
	return s.db.Create(whitelist).Error
}

func (s *WhitelistService) AddHostWhitelist(name, hostname, remark string) error {
	whitelist := &Whitelist{
		Name:     name,
		Hostname: hostname,
		Type:     WhitelistTypeHost,
		Remark:   remark,
	}
	return s.db.Create(whitelist).Error
}

func (s *WhitelistService) AddWhitelist(name, namespace, hostname string, whitelistType WhitelistType, remark string) error {
	whitelist := &Whitelist{
		Name:      name,
		Namespace: namespace,
		Hostname:  hostname,
		Type:      whitelistType,
		Remark:    remark,
	}
	return s.db.Create(whitelist).Error
}

func (s *WhitelistService) RemoveWhitelistByID(id uint) error {
	return s.db.Delete(&Whitelist{}, id).Error
}

func (s *WhitelistService) RemoveNamespaceWhitelist(namespace string) error {
	return s.db.Where("namespace = ? AND type = ?", namespace, WhitelistTypeNamespace).
		Delete(&Whitelist{}).Error
}

func (s *WhitelistService) RemoveHostWhitelist(hostname string) error {
	return s.db.Where("hostname = ? AND type = ?", hostname, WhitelistTypeHost).
		Delete(&Whitelist{}).Error
}

func (s *WhitelistService) UpdateWhitelist(id uint, name, namespace, hostname, remark string) error {
	updates := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"hostname":  hostname,
		"remark":    remark,
	}
	return s.db.Model(&Whitelist{}).Where("id = ?", id).Updates(updates).Error
}

func (s *WhitelistService) GetWhitelistByID(id uint) (*Whitelist, error) {
	var whitelist Whitelist
	err := s.db.First(&whitelist, id).Error
	if err != nil {
		return nil, err
	}
	return &whitelist, nil
}

func (s *WhitelistService) GetAllWhitelists() ([]Whitelist, error) {
	var whitelists []Whitelist
	err := s.db.Order("created_at desc").Find(&whitelists).Error
	return whitelists, err
}

func (s *WhitelistService) GetWhitelistsByType(whitelistType WhitelistType) ([]Whitelist, error) {
	var whitelists []Whitelist
	err := s.db.Where("type = ?", whitelistType).
		Order("created_at desc").
		Find(&whitelists).Error
	return whitelists, err
}

func (s *WhitelistService) GetNamespaceWhitelists() ([]Whitelist, error) {
	return s.GetWhitelistsByType(WhitelistTypeNamespace)
}

func (s *WhitelistService) GetHostWhitelists() ([]Whitelist, error) {
	return s.GetWhitelistsByType(WhitelistTypeHost)
}

func (s *WhitelistService) SearchWhitelists(keyword string) ([]Whitelist, error) {
	var whitelists []Whitelist
	err := s.db.Where("name LIKE ? OR namespace LIKE ? OR hostname LIKE ? OR remark LIKE ?",
		"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").
		Order("created_at desc").
		Find(&whitelists).Error
	return whitelists, err
}
