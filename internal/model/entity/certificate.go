package entity

import "time"

// Certificate is the GORM model for the certificates table.
type Certificate struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string `json:"name" gorm:"size:255;not null"`
	Domain        string `json:"domain" gorm:"size:255;not null"`
	Domains       string `json:"domains" gorm:"type:longtext"`
	Issuer        string `json:"issuer" gorm:"size:128;not null"`
	ProviderType  string `json:"provider_type" gorm:"size:32;not null"`
	KeyType       string `json:"key_type" gorm:"size:16"`
	KeyBits       int    `json:"key_bits"`
	CertPath      string `json:"cert_path" gorm:"size:512;not null"`
	KeyPath       string `json:"key_path" gorm:"size:512;not null"`
	FullchainPath string `json:"fullchain_path" gorm:"size:512"`
	CertContent   string `json:"cert_content" gorm:"type:longtext"`
	KeyContent    string `json:"key_content" gorm:"type:longtext"`
	// HasContent 只在**接口返回**里用 ✓（`gorm:"-"` 不入库 ✓）：面板是否**持有**这张证书的 PEM/私钥 ✓。
	//
	// 架构口径（用户口径 ✓）：**面板只管触发 ✓，证书由节点持有并由节点自己续 ✓，面板不落地 ✗**。
	// 所以节点上报的证书在面板上**只有元数据** ✓（域名/IP、签发者、到期、绑定节点、节点侧路径 ✓）
	// → `has_content=false` ✓：下载/分发这类需要 PEM 的操作要在界面上**明确说明** ✗，
	// 而不是让用户看到一个笼统的"操作失败" ✗。
	HasContent bool   `json:"has_content" gorm:"-"`
	Status     string `json:"status" gorm:"size:32;not null"`
	// ⚠ 不要加 `default:1` ✗：GORM 对"带 default 的字段"会**跳过零值** ✗ →
	// 代码里显式设成 0（例如上传的证书不该被自动续期）会被 DB 默认值改回 1 ✗
	// （真机实测：上传路径设 0，库里却是 1 ✗）。去掉 default 后零值能正常写入 ✓。
	AutoRenew int `json:"auto_renew" gorm:"not null"`
	// IssuerNodeID 是"**签发这张证书的节点**" ✓（0 = 面板本机签的 ✓）。
	//
	// 用户口径 ✓：**谁签发，就由谁续期** ✓；别的节点只是"**使用**"这张证书 ✓
	// （它们通过 node_certificates 关联 ✓，证书更新后由分发逻辑自动同步最新件 ✓）。
	// 之前续期会把"所有绑定节点"都问一遍 ✗ → 重复签发 + 重复消耗一次性 EAB ✗。
	//
	// ⚠ 不加 `not null` ✗（给已有表加 NOT NULL 列会被 SQLite 拒绝、导致面板起不来 ✓）。
	IssuerNodeID  int64      `json:"issuer_node_id" gorm:"index"`
	ExpiresAt     time.Time  `json:"expires_at" gorm:"not null"`
	LastRenewedAt *time.Time `json:"last_renewed_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"not null"`
}

func (Certificate) TableName() string { return "certificates" }
