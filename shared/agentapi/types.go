package agentapi

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

type Status struct {
	InstallReady     bool   `json:"install_ready"`
	Connected        bool   `json:"connected"`
	Docker           bool   `json:"docker"`
	Nginx            bool   `json:"nginx"`
	Certbot          bool   `json:"certbot"`
	RecoveryRequired bool   `json:"recovery_required"`
	Version          string `json:"version"`
}
type Container struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	State       string   `json:"state"`
	Status      string   `json:"status"`
	Managed     bool     `json:"managed"`
	CanManage   bool     `json:"can_manage"`
	CanReadLogs bool     `json:"can_read_logs"`
	Protected   bool     `json:"protected"`
	SiteID      string   `json:"site_id"`
	CPUPct      *float64 `json:"cpu_pct"`
	MemoryBytes *uint64  `json:"memory_bytes"`
}
type OperationRequest struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	ServerID   string          `json:"server_id"`
	ResourceID string          `json:"resource_id"`
	Payload    json.RawMessage `json:"payload"`
}
type Operation struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	State     string          `json:"state"`
	Phase     string          `json:"phase"`
	ErrorCode string          `json:"error_code"`
	Result    json.RawMessage `json:"result"`
	UpdatedAt time.Time       `json:"updated_at"`
}
type LogsRequest struct {
	ContainerID string `json:"container_id"`
	Tail        int    `json:"tail"`
}
type LogsResult struct {
	Lines     []string `json:"lines"`
	Truncated bool     `json:"truncated"`
}
type BackupFile struct {
	Name string
	Size int64
}
type Client interface {
	Status(context.Context) (Status, error)
	Containers(context.Context) ([]Container, error)
	Logs(context.Context, LogsRequest) (LogsResult, error)
	Submit(context.Context, OperationRequest) (Operation, error)
	Operation(context.Context, string) (Operation, error)
	Download(context.Context, string) (io.ReadCloser, BackupFile, error)
}
type ContainerInput struct {
	Action string `json:"action"`
}
type SiteInput struct {
	Name           string    `json:"name"`
	Domain         string    `json:"domain"`
	AdminEmail     string    `json:"admin_email"`
	AdminPassword  string    `json:"admin_password"`
	WordPressTitle string    `json:"wordpress_title"`
	Resources      Resources `json:"resources"`
}
type Resources struct {
	MemoryMB int64   `json:"memory_mb"`
	CPULimit float64 `json:"cpu_limit"`
}
type AdoptInput struct {
	Name                string   `json:"name"`
	Domain              string   `json:"domain"`
	WebContainerID      string   `json:"web_container_id"`
	DatabaseContainerID string   `json:"database_container_id"`
	WriterContainerIDs  []string `json:"writer_container_ids"`
	DatabaseSecretRef   string   `json:"database_secret_ref"`
}
type DomainInput struct {
	SiteID   string `json:"site_id"`
	Hostname string `json:"hostname"`
}
type BackupInput struct {
	ConfirmMaintenance bool   `json:"confirm_maintenance"`
	ConfirmDomain      string `json:"confirm_domain"`
	SiteID             string `json:"site_id"`
	BackupID           string `json:"backup_id"`
}
type SiteResources struct {
	Budget        Resources     `json:"resources"`
	PrimaryDomain *DomainResult `json:"primary_domain,omitempty"`
	SafetyBackup  *Backup       `json:"safety_backup,omitempty"`
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Domain        string        `json:"domain"`
	WebID         string        `json:"web_id"`
	DBID          string        `json:"db_id"`
	Writers       []string      `json:"writers"`
	Network       string        `json:"network"`
	WebVolume     string        `json:"web_volume"`
	DBVolume      string        `json:"db_volume"`
	Port          uint16        `json:"port"`
	SecretRef     string        `json:"secret_ref"`
	State         string        `json:"state"`
	Adopted       bool          `json:"adopted"`
}
type Certificate struct {
	ID          string `json:"id"`
	DomainID    string `json:"domain_id"`
	CertName    string `json:"cert_name"`
	Hostname    string `json:"hostname"`
	ExpiresAt   string `json:"expires_at"`
	Fingerprint string `json:"fingerprint"`
}
type DomainResult struct {
	ID          string       `json:"id"`
	SiteID      string       `json:"site_id"`
	Hostname    string       `json:"hostname"`
	DNSState    string       `json:"dns_state"`
	Certificate *Certificate `json:"certificate,omitempty"`
}
type Backup struct {
	ID        string `json:"id"`
	SiteID    string `json:"site_id"`
	SizeBytes int64  `json:"size_bytes"`
	Checksum  string `json:"checksum"`
	CreatedAt string `json:"created_at"`
	Version   int    `json:"version"`
}
