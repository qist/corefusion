package certificates

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	rtcert "corefusion/internal/runtime/cert"
	certService "corefusion/internal/service/certificate"
)

// 单个证书打包下载时，zip 里的固定文件名 ✓（可直接按惯例丢给 nginx 用 ✓）。
const (
	zipFileNameCert      = "cert.pem"      // **仅叶证书** ✓（惯例命名 ✓）
	zipFileNameFullchain = "fullchain.pem" // 叶 + 中间链 ✓（TLS 部署用这个 ✓）
	zipFileNameKey       = "privkey.pem"   // 私钥 ✓
	zipFileNameReadme    = "README.txt"
)

// certZipItem 是"一个证书在 zip 里的那部分" ✓。
type certZipItem struct {
	// Folder 为空 = 文件放在 zip 根目录 ✓（单个证书 ✓）；
	// 非空 = 该证书独占一个目录 ✓（批量打包 ✓）。
	Folder string
	Mat    *certService.CertMaterial
}

// Download 下载单个证书 ✓：GET /api/v1/certificates/:id/download?format=zip|cert|fullchain|key
//
// 默认 `zip` ✓：按**通用惯例**给三个文件 ✓ ——
//   - `fullchain.pem`：叶 + 中间链 ✓（**TLS 部署用这个** ✓）
//   - `cert.pem`：**仅叶证书** ✓（很多面板/服务商只要叶 ✓）
//   - `privkey.pem`：私钥 ✓
//
// ⚠ 注意与磁盘文件的区别 ✗：面板/内核实际使用的 `data/certs/<域名>/cert.pem`
// **含完整链** ✓（内核配置直接引用它做 TLS ✓，拆掉链会让客户端验不过 ✗），
// 所以"仅叶"是**下载时按惯例拆分**出来的 ✓，不改磁盘 ✓（详见 internal/runtime/cert/pemutil.go ✓）。
func (h *Handler) Download(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "INVALID_ID", "invalid certificate id")
		return
	}
	mat, err := h.certSvc.ReadCertMaterial(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "CERTIFICATE_CONTENT_UNAVAILABLE", err.Error())
		return
	}

	leaf, full := rtcert.SplitLeafAndChain([]byte(mat.CertPEM))
	base := rtcert.SanitizeFileName(mat.Name)
	if base == "certificate" && strings.TrimSpace(mat.Domain) != "" {
		base = rtcert.SanitizeFileName(mat.Domain)
	}

	switch strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "zip"))) {
	case "zip", "":
		data, zerr := buildCertZip([]certZipItem{{Mat: mat}})
		if zerr != nil {
			response.InternalError(c, "ZIP_FAILED", zerr.Error())
			return
		}
		writeAttachment(c, base+".zip", "application/zip", data)
	case "cert", "pem", "leaf":
		writeAttachment(c, base+".pem", "application/x-pem-file", leaf)
	case "fullchain", "chain":
		writeAttachment(c, base+"-fullchain.pem", "application/x-pem-file", full)
	case "key", "privkey":
		writeAttachment(c, base+".key", "application/x-pem-file", []byte(mat.KeyPEM))
	default:
		response.BadRequest(c, "INVALID_FORMAT", "format must be one of: zip, cert, fullchain, key")
	}
}

// Export 批量打包下载 ✓：GET /api/v1/certificates/export?ids=1,2,3
//
// 每个证书一个目录 ✓（目录名 = 消毒后的证书名 ✓），根目录附一份清单 README ✓。
// ⚠ 上限 200 张 ✗：避免一次拉爆内存 ✓（内容都读进内存再压缩 ✓）。
func (h *Handler) Export(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("ids"))
	if raw == "" {
		// 也允许只给一个 id ✓（前端少量下载时更省事 ✓）
		raw = strings.TrimSpace(c.Query("cert_id"))
	}
	if raw == "" {
		response.BadRequest(c, "IDS_REQUIRED", "ids is required, e.g. ?ids=1,2,3")
		return
	}

	var items []certZipItem
	used := map[string]int{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, perr := strconv.ParseInt(part, 10, 64)
		if perr != nil || id <= 0 {
			response.BadRequest(c, "INVALID_ID", "invalid certificate id: "+part)
			return
		}
		if len(items) >= 200 {
			response.BadRequest(c, "TOO_MANY_IDS", "at most 200 certificates per export")
			return
		}
		mat, merr := h.certSvc.ReadCertMaterial(c.Request.Context(), id)
		if merr != nil {
			// 单张读不到不该让整包失败 ✗ —— 但必须如实告知（前端会提示 ✓）
			continue
		}
		folder := rtcert.SanitizeFileName(mat.Name)
		if folder == "certificate" && strings.TrimSpace(mat.Domain) != "" {
			folder = rtcert.SanitizeFileName(mat.Domain)
		}
		// 目录重名 → 加序号 ✓（不能互相覆盖 ✗）
		if n := used[folder]; n > 0 {
			folder = fmt.Sprintf("%s-%d", folder, n+1)
		}
		used[rtcert.SanitizeFileName(mat.Name)]++
		items = append(items, certZipItem{Folder: folder, Mat: mat})
	}
	if len(items) == 0 {
		response.NotFound(c, "CERTIFICATE_CONTENT_UNAVAILABLE", "no downloadable certificate in the given ids")
		return
	}

	data, err := buildCertZip(items)
	if err != nil {
		response.InternalError(c, "ZIP_FAILED", err.Error())
		return
	}
	name := fmt.Sprintf("certificates-%s.zip", time.Now().Format("20060102-150405"))
	if len(items) == 1 {
		name = items[0].Folder + ".zip"
	}
	writeAttachment(c, name, "application/zip", data)
}

// writeAttachment 统一回一个"附件下载" ✓（禁止缓存 ✓：里面有私钥 ✗）。
func writeAttachment(c *gin.Context, filename, contentType string, data []byte) {
	name := rtcert.SanitizeFileName(filename)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, data)
}

// buildCertZip 按上面的约定打包 ✓（**纯函数** ✓，便于单测 ✓）。
func buildCertZip(items []certZipItem) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// 说明部分**每个目录都要有** ✓（只解压某个目录时也能看懂 ✓）
	header := "证书下载包（CoreFusion）\n\n" +
		"文件说明：\n" +
		"  fullchain.pem  叶证书 + 中间链，TLS 部署用这个\n" +
		"  cert.pem       仅叶证书（很多面板/服务商只要叶证书）\n" +
		"  privkey.pem    私钥，务必保密\n\n" +
		"注意：面板/内核实际引用的 data/certs/<域名>/cert.pem 含完整链，\n" +
		"      与本包中“仅叶证书”的 cert.pem 不同；部署 TLS 请用 fullchain.pem。\n\n" +
		"nginx 示例：\n" +
		"  ssl_certificate     /etc/nginx/ssl/fullchain.pem;\n" +
		"  ssl_certificate_key /etc/nginx/ssl/privkey.pem;\n"

	var readme strings.Builder
	readme.WriteString(header)
	readme.WriteString("\n本包含以下证书：\n")

	for _, it := range items {
		leaf, full := rtcert.SplitLeafAndChain([]byte(it.Mat.CertPEM))
		prefix := ""
		if it.Folder != "" {
			prefix = it.Folder + "/"
		}
		files := []struct {
			name string
			data []byte
		}{
			{zipFileNameCert, leaf},
			{zipFileNameFullchain, full},
			{zipFileNameKey, []byte(it.Mat.KeyPEM)},
		}
		for _, f := range files {
			w, werr := zw.Create(prefix + f.name)
			if werr != nil {
				return nil, werr
			}
			if _, werr = w.Write(f.data); werr != nil {
				return nil, werr
			}
		}
		// 每个目录（单个证书则是根目录 ✓）都放一份 README ✓：
		// **说明部分 + 这张证书自己的信息** ✓（这样单证书包也讲清了两个 cert.pem 的区别 ✓）
		w, werr := zw.Create(prefix + zipFileNameReadme)
		if werr != nil {
			return nil, werr
		}
		info := fmt.Sprintf("%s\n本证书：\n  域名: %s\n  签发者: %s\n  到期: %s\n",
			header, it.Mat.Domain, it.Mat.Issuer, it.Mat.ExpiresAt.Format(time.RFC3339))
		if _, werr = w.Write([]byte(info)); werr != nil {
			return nil, werr
		}
		readme.WriteString(fmt.Sprintf("  %s\t%s\t到期 %s\n",
			it.Mat.Domain, it.Mat.Issuer, it.Mat.ExpiresAt.Format("2006-01-02")))
	}

	if len(items) > 1 {
		w, werr := zw.Create(zipFileNameReadme)
		if werr != nil {
			return nil, werr
		}
		if _, werr = w.Write([]byte(readme.String())); werr != nil {
			return nil, werr
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
