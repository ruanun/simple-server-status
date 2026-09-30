package api

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

func TestMonthlyCost(t *testing.T) {
	list := []store.Server{
		{Price: f64(10), Currency: "$", BillingCycle: "monthly"},
		{Price: f64(30), Currency: "$", BillingCycle: "quarterly"},
		{Price: f64(120), Currency: "¥", BillingCycle: "yearly"},
		{Price: f64(99), Currency: "$", BillingCycle: "once"},
		{Price: f64(5), Currency: "$", BillingCycle: ""},
		{Currency: "$", BillingCycle: "monthly"},
		{Price: f64(10), Currency: "", BillingCycle: "yearly"},
	}
	want := []costGroup{{Currency: "", Amount: 0.83, Servers: 1}, {Currency: "$", Amount: 20, Servers: 2}, {Currency: "¥", Amount: 10, Servers: 1}}
	if got := monthlyCost(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("得到 %+v", got)
	}
}

func TestExpiringServers(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	day := int64(86400)
	list := []store.Server{
		{ID: "a", Name: "a", ExpireAt: i64(now.Unix() + 5*day)},
		{ID: "b", Name: "b", ExpireAt: i64(now.Unix() + 31*day)},
		{ID: "c", Name: "c", ExpireAt: i64(now.Unix() - 2*day)},
		{ID: "d", Name: "d", ExpireAt: i64(now.Unix() - 40*day)},
		{ID: "e", Name: "e"},
		{ID: "f", Name: "f", ExpireAt: i64(now.Unix() + 30*day)},
	}
	got := expiringServers(list, now)
	ids := []string{}
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if !reflect.DeepEqual(ids, []string{"c", "a", "f"}) || got[0].Days != -2 || got[1].Days != 5 {
		t.Fatalf("得到 %+v", got)
	}
}

func TestAdminOverviewEndpoint(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "a", Price: f64(12), Currency: "$", BillingCycle: "yearly", ExpireAt: i64(e.clock.Now().Unix() + 86400)})
	if code, _ := e.do("GET", "/api/admin/overview", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", code)
	}
	code, body := e.do("GET", "/api/admin/overview", e.adminToken(), nil)
	got := decodeData[overview](t, body)
	if code != http.StatusOK || len(got.MonthlyCost) != 1 || got.MonthlyCost[0].Amount != 1 || len(got.Expiring) != 1 || got.Expiring[0].Days != 1 {
		t.Fatalf("%d %+v", code, got)
	}
}
