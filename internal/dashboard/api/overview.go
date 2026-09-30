package api

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// costGroup 某币种的月均费用
type costGroup struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
	Servers  int     `json:"servers"`
}

// expiringServer 即将到期（或刚过期）的服务器
type expiringServer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ExpireAt int64  `json:"expire_at"`
	Days     int    `json:"days"`
}

type overview struct {
	MonthlyCost []costGroup      `json:"monthly_cost"`
	Expiring    []expiringServer `json:"expiring"`
}

// monthlyDivisor 付费周期折算为月的除数；一次性与未设置周期不计入月均费用
var monthlyDivisor = map[string]float64{"monthly": 1, "quarterly": 3, "yearly": 12}

// expiringWindow 总览列出的到期范围（天）：未来 30 天内到期与过期不超过 30 天
const expiringWindow = 30

// monthlyCost 按币种汇总月均费用（保留 2 位小数），币种按字符串升序
func monthlyCost(list []store.Server) []costGroup {
	byCur := map[string]*costGroup{}
	for _, s := range list {
		div, ok := monthlyDivisor[s.BillingCycle]
		if !ok || s.Price == nil {
			continue
		}
		g := byCur[s.Currency]
		if g == nil {
			g = &costGroup{Currency: s.Currency}
			byCur[s.Currency] = g
		}
		g.Amount += *s.Price / div
		g.Servers++
	}
	out := make([]costGroup, 0, len(byCur))
	for _, g := range byCur {
		g.Amount = math.Round(g.Amount*100) / 100
		out = append(out, *g)
	}
	slices.SortFunc(out, func(a, b costGroup) int { return strings.Compare(a.Currency, b.Currency) })
	return out
}

// expiringServers 返回到期范围内的服务器，按剩余天数升序、同天按名称
func expiringServers(list []store.Server, now time.Time) []expiringServer {
	out := []expiringServer{}
	for _, s := range list {
		if s.ExpireAt == nil {
			continue
		}
		days := int(math.Ceil(float64(*s.ExpireAt-now.Unix()) / 86400))
		if days > expiringWindow || days < -expiringWindow {
			continue
		}
		out = append(out, expiringServer{ID: s.ID, Name: s.Name, ExpireAt: *s.ExpireAt, Days: days})
	}
	slices.SortFunc(out, func(a, b expiringServer) int {
		if c := cmp.Compare(a.Days, b.Days); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func (a *API) adminOverview(c *gin.Context) {
	list := a.serverList()
	respond(c, overview{MonthlyCost: monthlyCost(list), Expiring: expiringServers(list, a.Now())})
}
