package certificates

import "testing"

// 核心回归 ✓：下发给节点的密钥规格必须"**请求优先、库记录兜底**" ✗ ——
//
// 以前指令里**根本没有** key_type/key_bits ✗ → 节点侧 `resolveKeyType("",0)`
// 回退 **EC256** ✗ → 库里登记 `rsa/4096` 的证书，每次续签/续期都被"续"成 EC ✗
// （真机现象：记录 rsa、落地文件却是 `EC PRIVATE KEY` ✓）。
func TestResolveIssueKeyType(t *testing.T) {
	cases := []struct {
		name            string
		reqType, dbType string
		reqBits, dbBits int
		wantType        string
		wantBits        int
	}{
		{
			// 表单上明确选了 → 以**请求**为准 ✓（哪怕和库记录不一致 ✓）
			name: "请求优先于库记录", reqType: "ecc", reqBits: 384, dbType: "rsa", dbBits: 4096,
			wantType: "ecc", wantBits: 384,
		},
		{
			// ★ 关键用例 ✗：前端不传（续签常见 ✓）→ 必须用库里的 rsa/4096 ✓，
			// 否则节点签 EC256 ✓ → "rsa 证书被续成 EC" ✗
			name: "库记录兜底（rsa 不会被续成 EC）", reqType: "", reqBits: 0, dbType: "rsa", dbBits: 4096,
			wantType: "rsa", wantBits: 4096,
		},
		{
			// 只给类型不给位数 ✓ → 位数用库里的 ✓（不要把 4096 降成默认 ✗）
			name: "类型给、位数兜底", reqType: "rsa", reqBits: 0, dbType: "rsa", dbBits: 4096,
			wantType: "rsa", wantBits: 4096,
		},
		{
			// 只给位数 ✓ → 类型用库里的 ✓
			name: "位数给、类型兜底", reqType: "", reqBits: 2048, dbType: "rsa", dbBits: 4096,
			wantType: "rsa", wantBits: 2048,
		},
		{
			// 两边都没有 → 保持空/0 ✓ = 节点按历史默认 EC256 ✓（与改动前行为一致 ✓，不倒退 ✓）
			name: "都为空 → 留空 = 节点默认", reqType: "", reqBits: 0, dbType: "", dbBits: 0,
			wantType: "", wantBits: 0,
		},
		{
			// 空白串等同没传 ✓（前端常传 " " ✓）
			name: "空白串当没传", reqType: "  ", reqBits: 0, dbType: " rsa ", dbBits: 4096,
			wantType: "rsa", wantBits: 4096,
		},
		{
			// 脏数据防御：负数不该原样下发 ✗
			name: "负数位数归零", reqType: "", reqBits: -1, dbType: "rsa", dbBits: -4096,
			wantType: "rsa", wantBits: 0,
		},
	}

	for _, c := range cases {
		gotType, gotBits := resolveIssueKeyType(c.reqType, c.reqBits, c.dbType, c.dbBits)
		if gotType != c.wantType || gotBits != c.wantBits {
			t.Fatalf("%s ✗：resolveIssueKeyType(%q,%d,%q,%d) = (%q,%d)，应为 (%q,%d) ✓",
				c.name, c.reqType, c.reqBits, c.dbType, c.dbBits,
				gotType, gotBits, c.wantType, c.wantBits)
		}
	}
}
