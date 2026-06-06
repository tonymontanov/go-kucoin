/*
FILE: affiliate/affiliate.go

DESCRIPTION:
Read-only affiliate reports.

ENDPOINTS:
  - GET /api/v2/affiliate/queryMyCommission   GetCommission
  - GET /api/v2/affiliate/inviter/statistics  GetInviterRebate (deprecated)
*/

package affiliate

import (
	"context"

	"github.com/shopspring/decimal"

	afftypes "github.com/tonymontanov/go-kucoin/v2/affiliate/types"
)

// GetCommission returns the caller's settled-commission records.
func (c *Client) GetCommission(ctx context.Context, q afftypes.CommissionQuery) ([]afftypes.Commission, error) {
	var query map[string]string = map[string]string{}
	if q.SiteType != "" {
		query["siteType"] = q.SiteType
	}
	if q.RebateType != "" {
		query["rebateType"] = q.RebateType
	}
	if q.RebateFrom > 0 {
		query["rebateStartAt"] = itoa64(q.RebateFrom)
	}
	if q.RebateTo > 0 {
		query["rebateEndAt"] = itoa64(q.RebateTo)
	}
	if q.Page > 0 {
		query["page"] = itoa(q.Page)
	}
	if q.PageSize > 0 {
		query["pageSize"] = itoa(q.PageSize)
	}
	if q.UserID != "" {
		query["userId"] = q.UserID
	}
	if q.DataType != "" {
		query["dataType"] = q.DataType
	}
	var wire []commissionWire
	if err := c.doGET(ctx, "/api/v2/affiliate/queryMyCommission", query, &wire); err != nil {
		return nil, err
	}
	var out []afftypes.Commission = make([]afftypes.Commission, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toCommission()
	}
	return out, nil
}

// GetInviterRebate returns inviter rebate statistics for a given date
// ("YYYYMMDD"). DEPRECATED by KuCoin ("Get Account"); kept for completeness.
func (c *Client) GetInviterRebate(ctx context.Context, date, offset string, maxCount int) ([]afftypes.Rebate, error) {
	var query map[string]string = map[string]string{}
	if date != "" {
		query["date"] = date
	}
	if offset != "" {
		query["offset"] = offset
	}
	if maxCount > 0 {
		query["maxCount"] = itoa(maxCount)
	}
	var wire []rebateWire
	if err := c.doGET(ctx, "/api/v2/affiliate/inviter/statistics", query, &wire); err != nil {
		return nil, err
	}
	var out []afftypes.Rebate = make([]afftypes.Rebate, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toRebate()
	}
	return out, nil
}

type commissionWire struct {
	SiteType        string          `json:"siteType"`
	RebateType      int             `json:"rebateType"`
	PayoutTime      int64           `json:"payoutTime"`
	PeriodStartTime int64           `json:"periodStartTime"`
	PeriodEndTime   int64           `json:"periodEndTime"`
	Status          int             `json:"status"`
	TakerVolume     decimal.Decimal `json:"takerVolume"`
	MakerVolume     decimal.Decimal `json:"makerVolume"`
	Commission      decimal.Decimal `json:"commission"`
	Currency        string          `json:"currency"`
}

func (w commissionWire) toCommission() afftypes.Commission {
	return afftypes.Commission{
		SiteType:        w.SiteType,
		RebateType:      w.RebateType,
		PayoutTime:      w.PayoutTime,
		PeriodStartTime: w.PeriodStartTime,
		PeriodEndTime:   w.PeriodEndTime,
		Status:          w.Status,
		TakerVolume:     w.TakerVolume,
		MakerVolume:     w.MakerVolume,
		Commission:      w.Commission,
		Currency:        w.Currency,
	}
}

type rebateWire struct {
	M1UID    string          `json:"m1Uid"`
	RCode    string          `json:"rcode"`
	M2UID    string          `json:"m2Uid"`
	Amount   decimal.Decimal `json:"amount"`
	Rebate   decimal.Decimal `json:"rebate"`
	CashBack decimal.Decimal `json:"cashBack"`
	Offset   string          `json:"offset"`
}

func (w rebateWire) toRebate() afftypes.Rebate {
	return afftypes.Rebate{
		M1UID:    w.M1UID,
		RCode:    w.RCode,
		M2UID:    w.M2UID,
		Amount:   w.Amount,
		Rebate:   w.Rebate,
		CashBack: w.CashBack,
		Offset:   w.Offset,
	}
}
