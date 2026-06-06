/*
FILE: viplending/viplending.go

DESCRIPTION:
The three read-only VIP Lending (OTC loan) queries + wire→type converters.

ENDPOINTS:
  - GET /api/v1/otc-loan/discount-rate-configs  gradient collateral rates
  - GET /api/v1/otc-loan/loan                    consolidated loan position
  - GET /api/v1/otc-loan/accounts                participating accounts
*/

package viplending

import (
	"context"

	"github.com/shopspring/decimal"

	viptypes "github.com/tonymontanov/go-kucoin/v2/viplending/types"
)

// GetCollateralConfigs returns the gradient collateral (discount) rate per
// currency.
func (c *Client) GetCollateralConfigs(ctx context.Context) ([]viptypes.DiscountRateConfig, error) {
	var wire []discountConfigWire
	if err := c.doGET(ctx, "/api/v1/otc-loan/discount-rate-configs", nil, &wire); err != nil {
		return nil, err
	}
	var out []viptypes.DiscountRateConfig = make([]viptypes.DiscountRateConfig, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toConfig()
	}
	return out, nil
}

// GetLoanInfo returns the caller's consolidated OTC loan position. The position
// fields are zero-valued when the account has no active loan.
func (c *Client) GetLoanInfo(ctx context.Context) (*viptypes.LoanInfo, error) {
	var wire loanInfoWire
	if err := c.doGET(ctx, "/api/v1/otc-loan/loan", nil, &wire); err != nil {
		return nil, err
	}
	var info viptypes.LoanInfo = wire.toLoanInfo()
	return &info, nil
}

// GetAccounts returns the accounts participating in OTC lending.
func (c *Client) GetAccounts(ctx context.Context) ([]viptypes.LendingAccount, error) {
	var wire []lendingAccountWire
	if err := c.doGET(ctx, "/api/v1/otc-loan/accounts", nil, &wire); err != nil {
		return nil, err
	}
	var out []viptypes.LendingAccount = make([]viptypes.LendingAccount, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toAccount()
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type discountLevelWire struct {
	Left         int64           `json:"left"`
	Right        int64           `json:"right"`
	DiscountRate decimal.Decimal `json:"discountRate"`
}

type discountConfigWire struct {
	Currency   string              `json:"currency"`
	UsdtLevels []discountLevelWire `json:"usdtLevels"`
}

func (w discountConfigWire) toConfig() viptypes.DiscountRateConfig {
	var levels []viptypes.DiscountLevel = make([]viptypes.DiscountLevel, len(w.UsdtLevels))
	var i int
	for i = 0; i < len(w.UsdtLevels); i++ {
		levels[i] = viptypes.DiscountLevel{
			Left:         w.UsdtLevels[i].Left,
			Right:        w.UsdtLevels[i].Right,
			DiscountRate: w.UsdtLevels[i].DiscountRate,
		}
	}
	return viptypes.DiscountRateConfig{Currency: w.Currency, UsdtLevels: levels}
}

type loanOrderWire struct {
	OrderID   string          `json:"orderId"`
	Currency  string          `json:"currency"`
	Principal decimal.Decimal `json:"principal"`
	Interest  decimal.Decimal `json:"interest"`
}

type ltvWire struct {
	TransferLtv           decimal.Decimal `json:"transferLtv"`
	OnlyClosePosLtv       decimal.Decimal `json:"onlyClosePosLtv"`
	DelayedLiquidationLtv decimal.Decimal `json:"delayedLiquidationLtv"`
	InstantLiquidationLtv decimal.Decimal `json:"instantLiquidationLtv"`
	CurrentLtv            decimal.Decimal `json:"currentLtv"`
}

type marginAssetWire struct {
	MarginCcy    string          `json:"marginCcy"`
	MarginQty    decimal.Decimal `json:"marginQty"`
	MarginFactor decimal.Decimal `json:"marginFactor"`
}

type loanInfoWire struct {
	ParentUID            string            `json:"parentUid"`
	Orders               []loanOrderWire   `json:"orders"`
	Ltv                  ltvWire           `json:"ltv"`
	TotalMarginAmount    decimal.Decimal   `json:"totalMarginAmount"`
	TransferMarginAmount decimal.Decimal   `json:"transferMarginAmount"`
	Margins              []marginAssetWire `json:"margins"`
}

func (w loanInfoWire) toLoanInfo() viptypes.LoanInfo {
	var orders []viptypes.LoanOrder = make([]viptypes.LoanOrder, len(w.Orders))
	var i int
	for i = 0; i < len(w.Orders); i++ {
		orders[i] = viptypes.LoanOrder{
			OrderID:   w.Orders[i].OrderID,
			Currency:  w.Orders[i].Currency,
			Principal: w.Orders[i].Principal,
			Interest:  w.Orders[i].Interest,
		}
	}
	var margins []viptypes.MarginAsset = make([]viptypes.MarginAsset, len(w.Margins))
	for i = 0; i < len(w.Margins); i++ {
		margins[i] = viptypes.MarginAsset{
			MarginCcy:    w.Margins[i].MarginCcy,
			MarginQty:    w.Margins[i].MarginQty,
			MarginFactor: w.Margins[i].MarginFactor,
		}
	}
	return viptypes.LoanInfo{
		ParentUID: w.ParentUID,
		Orders:    orders,
		Ltv: viptypes.LTV{
			TransferLtv:           w.Ltv.TransferLtv,
			OnlyClosePosLtv:       w.Ltv.OnlyClosePosLtv,
			DelayedLiquidationLtv: w.Ltv.DelayedLiquidationLtv,
			InstantLiquidationLtv: w.Ltv.InstantLiquidationLtv,
			CurrentLtv:            w.Ltv.CurrentLtv,
		},
		TotalMarginAmount:    w.TotalMarginAmount,
		TransferMarginAmount: w.TransferMarginAmount,
		Margins:              margins,
	}
}

type lendingAccountWire struct {
	UID          string          `json:"uid"`
	MarginCcy    string          `json:"marginCcy"`
	MarginQty    decimal.Decimal `json:"marginQty"`
	MarginFactor decimal.Decimal `json:"marginFactor"`
	AccountType  string          `json:"accountType"`
	IsParent     bool            `json:"isParent"`
}

func (w lendingAccountWire) toAccount() viptypes.LendingAccount {
	return viptypes.LendingAccount{
		UID:          w.UID,
		MarginCcy:    w.MarginCcy,
		MarginQty:    w.MarginQty,
		MarginFactor: w.MarginFactor,
		AccountType:  w.AccountType,
		IsParent:     w.IsParent,
	}
}
