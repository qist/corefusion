package entity

import "time"

// NodeCertificate 是"节点 ↔ 证书"的**多对多**绑定 ✓。
//
// 为什么必须是一张表 ✗：`node_servers.certificate_id` 只能放**一张**证书 ✗，
// 而一个节点实际上常常要同时用多张（典型：一张 **IP 证书** + 一张**域名证书** ✓）——
// 续期、分发、界面勾选都得按"这个节点的**全部**证书"来处理 ✓。
//
// `node_servers.certificate_id` 继续保留为**主证书** ✓（兼容既有的代理配置与分发逻辑 ✓：
// 它们只需要知道"这张节点当前主要用哪张" ✓）。
type NodeCertificate struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeServerID  int64     `json:"node_server_id" gorm:"not null;uniqueIndex:uk_node_cert,priority:1"`
	CertificateID int64     `json:"certificate_id" gorm:"not null;uniqueIndex:uk_node_cert,priority:2"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
}

// TableName 指定表名。
func (NodeCertificate) TableName() string { return "node_certificates" }
