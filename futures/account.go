/*
FILE: futures/account.go

DESCRIPTION:
Signed account / position sub-client for the KuCoin Futures profile:

  - GET /api/v1/account-overview?currency=   account summary (one currency)
  - GET /api/v1/positions                      all open positions
  - GET /api/v1/position?symbol=               one position

GetAccountOverview returns the KuCoin-shaped AccountOverview; GetBalance
adapts it into the protocol-common root types.Balance so desk code can stay
profile-agnostic.
*/

package futures

import (
	"context"

	"github.com/shopspring/decimal"

	futurestypes "github.com/tonymontanov/go-kucoin/v2/futures/types"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
	roottypes "github.com/tonymontanov/go-kucoin/v2/types"
)

// AccountClient — signed account / position sub-client.
type AccountClient struct {
	c *Client
}

// newAccountClient wires the sub-client to its parent.
func newAccountClient(c *Client) *AccountClient {
	return &AccountClient{c: c}
}

// accountMeta is the rate-limit metadata for account queries.
var accountMeta = rest.RequestMeta{Category: "query"}

// GetAccountOverview returns the futures account summary for the given
// settle currency (e.g. "USDT", "XBT"). Empty currency lets KuCoin apply
// its default (XBT).
func (a *AccountClient) GetAccountOverview(ctx context.Context, currency string) (*futurestypes.AccountOverview, error) {
	var query map[string]string
	if currency != "" {
		query = map[string]string{"currency": currency}
	}
	var wire accountOverviewWire
	if err := a.c.doGET(ctx, true, "/api/v1/account-overview", query, accountMeta, &wire); err != nil {
		return nil, err
	}
	var ov futurestypes.AccountOverview = wire.toOverview()
	return &ov, nil
}

// GetBalance returns the account state as the protocol-common Balance.
func (a *AccountClient) GetBalance(ctx context.Context, currency string) (*roottypes.Balance, error) {
	var ov *futurestypes.AccountOverview
	var err error
	ov, err = a.GetAccountOverview(ctx, currency)
	if err != nil {
		return nil, err
	}
	var locked decimal.Decimal = ov.PositionMargin.Add(ov.OrderMargin)
	var bal roottypes.Balance = roottypes.Balance{
		MarginCoin:        ov.Currency,
		TotalEquity:       ov.AccountEquity,
		AvailableBalance:  ov.AvailableBalance,
		LockedBalance:     locked,
		UnrealizedPnL:     ov.UnrealizedPnL,
		MaintenanceMargin: decimal.Zero,
		Coins: []roottypes.CoinBalance{
			{
				Coin:           ov.Currency,
				Equity:         ov.AccountEquity,
				Available:      ov.AvailableBalance,
				OrderMargin:    ov.OrderMargin,
				PositionMargin: ov.PositionMargin,
				FrozenFunds:    ov.FrozenFunds,
				UnrealizedPnL:  ov.UnrealizedPnL,
			},
		},
	}
	return &bal, nil
}

// GetPositions returns every open position on the account.
func (a *AccountClient) GetPositions(ctx context.Context) ([]futurestypes.PositionInfo, error) {
	var wire []positionWire
	if err := a.c.doGET(ctx, true, "/api/v1/positions", nil, accountMeta, &wire); err != nil {
		return nil, err
	}
	var out []futurestypes.PositionInfo = make([]futurestypes.PositionInfo, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toPositionInfo()
	}
	return out, nil
}

// GetPosition returns the position for a single contract.
func (a *AccountClient) GetPosition(ctx context.Context, symbol string) (*futurestypes.PositionInfo, error) {
	if symbol == "" {
		return nil, errInvalidRequest("GetPosition", "symbol is required")
	}
	var wire positionWire
	if err := a.c.doGET(ctx, true, "/api/v1/position", map[string]string{"symbol": symbol}, accountMeta, &wire); err != nil {
		return nil, err
	}
	var info futurestypes.PositionInfo = wire.toPositionInfo()
	return &info, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

// accountOverviewWire mirrors /api/v1/account-overview. KuCoin spells the
// PnL field "unrealisedPNL".
type accountOverviewWire struct {
	Currency         string          `json:"currency"`
	AccountEquity    decimal.Decimal `json:"accountEquity"`
	UnrealisedPNL    decimal.Decimal `json:"unrealisedPNL"`
	MarginBalance    decimal.Decimal `json:"marginBalance"`
	PositionMargin   decimal.Decimal `json:"positionMargin"`
	OrderMargin      decimal.Decimal `json:"orderMargin"`
	FrozenFunds      decimal.Decimal `json:"frozenFunds"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
}

func (w accountOverviewWire) toOverview() futurestypes.AccountOverview {
	return futurestypes.AccountOverview{
		Currency:         w.Currency,
		AccountEquity:    w.AccountEquity,
		MarginBalance:    w.MarginBalance,
		AvailableBalance: w.AvailableBalance,
		PositionMargin:   w.PositionMargin,
		OrderMargin:      w.OrderMargin,
		FrozenFunds:      w.FrozenFunds,
		UnrealizedPnL:    w.UnrealisedPNL,
	}
}

// positionWire mirrors /api/v1/position(s). KuCoin spells PnL fields with
// British "unrealised". crossMode true → CROSS margin.
type positionWire struct {
	Symbol            string          `json:"symbol"`
	SettleCurrency    string          `json:"settleCurrency"`
	IsOpen            bool            `json:"isOpen"`
	CrossMode         bool            `json:"crossMode"`
	CurrentQty        decimal.Decimal `json:"currentQty"`
	AvgEntryPrice     decimal.Decimal `json:"avgEntryPrice"`
	MarkPrice         decimal.Decimal `json:"markPrice"`
	MarkValue         decimal.Decimal `json:"markValue"`
	LiquidationPrice  decimal.Decimal `json:"liquidationPrice"`
	BankruptPrice     decimal.Decimal `json:"bankruptPrice"`
	RealLeverage      decimal.Decimal `json:"realLeverage"`
	PosMargin         decimal.Decimal `json:"posMargin"`
	PosCost           decimal.Decimal `json:"posCost"`
	MaintMarginReq    decimal.Decimal `json:"maintMarginReq"`
	RiskLimit         decimal.Decimal `json:"riskLimit"`
	UnrealisedPnl     decimal.Decimal `json:"unrealisedPnl"`
	UnrealisedPnlPcnt decimal.Decimal `json:"unrealisedPnlPcnt"`
	RealisedPnl       decimal.Decimal `json:"realisedPnl"`
	OpeningTimestamp  int64           `json:"openingTimestamp"`
}

func (w positionWire) toPositionInfo() futurestypes.PositionInfo {
	var mode futurestypes.MarginMode = futurestypes.MarginIsolated
	if w.CrossMode {
		mode = futurestypes.MarginCross
	}
	return futurestypes.PositionInfo{
		Symbol:             w.Symbol,
		SettleCurrency:     w.SettleCurrency,
		IsOpen:             w.IsOpen,
		CrossMode:          w.CrossMode,
		MarginMode:         mode,
		CurrentQty:         w.CurrentQty,
		CurrentQtyKnown:    true, // REST snapshot всегда несёт currentQty.
		AvgEntryPrice:      w.AvgEntryPrice,
		MarkPrice:          w.MarkPrice,
		MarkValue:          w.MarkValue,
		LiquidationPrice:   w.LiquidationPrice,
		BankruptPrice:      w.BankruptPrice,
		RealLeverage:       w.RealLeverage,
		PosMargin:          w.PosMargin,
		PosCost:            w.PosCost,
		MaintMarginReq:     w.MaintMarginReq,
		RiskLimit:          w.RiskLimit,
		UnrealizedPnL:      w.UnrealisedPnl,
		UnrealizedPnLPct:   w.UnrealisedPnlPcnt,
		RealizedPnL:        w.RealisedPnl,
		OpeningTimestampMs: w.OpeningTimestamp,
	}
}
