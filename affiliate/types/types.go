/*
FILE: affiliate/types/types.go

DESCRIPTION:
Types for the KuCoin Affiliate profile, mapped from:

  - GET /api/v2/affiliate/queryMyCommission     → []Commission
  - GET /api/v2/affiliate/inviter/statistics    → []Rebate (deprecated)
*/

package types

import "github.com/shopspring/decimal"

// CommissionQuery — filters for the my-commission report (all optional).
type CommissionQuery struct {
	SiteType   string
	RebateType string
	RebateFrom int64 // rebateStartAt, ms epoch
	RebateTo   int64 // rebateEndAt, ms epoch
	Page       int
	PageSize   int
	UserID     string
	DataType   string
}

// Commission — one settled-commission record.
type Commission struct {
	SiteType        string
	RebateType      int
	PayoutTime      int64
	PeriodStartTime int64
	PeriodEndTime   int64
	Status          int
	TakerVolume     decimal.Decimal
	MakerVolume     decimal.Decimal
	Commission      decimal.Decimal
	Currency        string
}

// Rebate — one inviter rebate record (deprecated "Get Account" endpoint).
type Rebate struct {
	M1UID    string
	RCode    string
	M2UID    string
	Amount   decimal.Decimal
	Rebate   decimal.Decimal
	CashBack decimal.Decimal
	Offset   string
}
