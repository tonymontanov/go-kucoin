/*
FILE: account/fee.go

DESCRIPTION:
Signed fee sub-client: account base spot/margin rate and actual per-symbol
trade fees.

ENDPOINTS:
  - GET /api/v1/base-fee    account base spot/margin maker/taker rate
  - GET /api/v1/trade-fees  actual per-symbol maker/taker rates (≤10 symbols)
*/

package account

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"

	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

// FeeClient — signed trade-fee sub-client.
type FeeClient struct {
	c *Client
}

func newFeeClient(c *Client) *FeeClient { return &FeeClient{c: c} }

// GetBaseFee returns the account's base spot/margin maker/taker rates.
// currencyType selects the fee currency basis: 0 (crypto, default), 1 (USDT)
// or 2 (KCS). Pass a negative value to omit the parameter.
func (f *FeeClient) GetBaseFee(ctx context.Context, currencyType int) (*accounttypes.BaseFee, error) {
	var query map[string]string
	if currencyType >= 0 {
		query = map[string]string{"currencyType": itoa(currencyType)}
	}
	var wire feeWire
	if err := f.c.doGET(ctx, true, "/api/v1/base-fee", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	return &accounttypes.BaseFee{
		TakerFeeRate: wire.TakerFeeRate,
		MakerFeeRate: wire.MakerFeeRate,
	}, nil
}

// GetTradeFees returns the actual maker/taker rates for up to 10 symbols.
func (f *FeeClient) GetTradeFees(ctx context.Context, symbols []string) ([]accounttypes.TradeFee, error) {
	if len(symbols) == 0 {
		return nil, errInvalidRequest("GetTradeFees", "at least one symbol is required")
	}
	if len(symbols) > 10 {
		return nil, errInvalidRequest("GetTradeFees", "at most 10 symbols per call")
	}
	var query map[string]string = map[string]string{"symbols": strings.Join(symbols, ",")}
	var wire []tradeFeeWire
	if err := f.c.doGET(ctx, true, "/api/v1/trade-fees", query, queryMeta, &wire); err != nil {
		return nil, err
	}
	var out []accounttypes.TradeFee = make([]accounttypes.TradeFee, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = accounttypes.TradeFee{
			Symbol:       wire[i].Symbol,
			TakerFeeRate: wire[i].TakerFeeRate,
			MakerFeeRate: wire[i].MakerFeeRate,
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------
// Wire structs.
// ---------------------------------------------------------------------

type feeWire struct {
	TakerFeeRate decimal.Decimal `json:"takerFeeRate"`
	MakerFeeRate decimal.Decimal `json:"makerFeeRate"`
}

type tradeFeeWire struct {
	Symbol       string          `json:"symbol"`
	TakerFeeRate decimal.Decimal `json:"takerFeeRate"`
	MakerFeeRate decimal.Decimal `json:"makerFeeRate"`
}
