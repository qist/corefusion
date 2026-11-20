package charge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 充值订单列表按类型筛选：套餐购买(purchase) 与充值(charge) 必须能分开查，
// 否则财务对账时两者混在一页。
func TestListOrdersFilterByOrderType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "orders.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.Client{}, &entity.ChargeOrder{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	client := &entity.Client{Email: "filter@test.local", Password: "x", Status: "active"}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	orders := []entity.ChargeOrder{
		{ClientID: client.ID, TradeNo: "T-CHARGE-1", Source: "site", Amount: 1000, Currency: "CNY", OrderType: entity.ChargeOrderTypeCharge, Status: entity.ChargeStatusPaid},
		{ClientID: client.ID, TradeNo: "T-PURCHASE-1", Source: "site", Amount: 990, Currency: "CNY", OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPaid},
		{ClientID: client.ID, TradeNo: "T-CHARGE-2", Source: "site", Amount: 500, Currency: "CNY", OrderType: entity.ChargeOrderTypeCharge, Status: entity.ChargeStatusPending},
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("create order: %v", err)
		}
	}

	h := &Handler{db: db, clientRepo: repository.NewClientRepository(db)}
	list := func(query string) (int64, []string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/charge-orders"+query, nil)
		h.ListOrders(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body=%s", query, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Items []struct {
					TradeNo string `json:"trade_no"`
				} `json:"items"`
				Total int64 `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode: %v (%s)", query, err, w.Body.String())
		}
		trades := make([]string, 0, len(resp.Data.Items))
		for _, it := range resp.Data.Items {
			trades = append(trades, it.TradeNo)
		}
		return resp.Data.Total, trades
	}

	total, trades := list("")
	if total != 3 || len(trades) != 3 {
		t.Fatalf("不带筛选应返回 3 条，实际 total=%d items=%v", total, trades)
	}

	total, trades = list("?order_type=purchase")
	if total != 1 || len(trades) != 1 || trades[0] != "T-PURCHASE-1" {
		t.Fatalf("purchase 应只返回 1 条套餐订单，实际 total=%d items=%v", total, trades)
	}

	total, trades = list("?order_type=charge")
	if total != 2 || len(trades) != 2 {
		t.Fatalf("charge 应返回 2 条充值订单，实际 total=%d items=%v", total, trades)
	}
	for _, tn := range trades {
		if tn == "T-PURCHASE-1" {
			t.Fatalf("charge 结果里不应出现套餐订单: %v", trades)
		}
	}

	// 类型 + 状态组合筛选
	total, trades = list("?order_type=charge&status=pending")
	if total != 1 || len(trades) != 1 || trades[0] != "T-CHARGE-2" {
		t.Fatalf("组合筛选应只返回 1 条，实际 total=%d items=%v", total, trades)
	}

	// 未知类型不应报错，只是查不到
	total, _ = list("?order_type=bogus")
	if total != 0 {
		t.Fatalf("未知类型应返回 0 条，实际 %d", total)
	}
}
