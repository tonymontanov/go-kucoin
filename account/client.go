/*
FILE: account/client.go

DESCRIPTION:
Root sub-client for the KuCoin Account & Funding profile (v2.5). Holds a
reference to the parent kucoin.Client (signer, logger, config) and a DEDICATED
REST client bound to the spot host (api.kucoin.com) — the account/funding
surface lives on the spot host, while the root REST client defaults to the
futures host. Exposes the domain sub-clients: Account, Deposit, Withdrawal,
Transfer, Fee, Currency.

CONTRACT:
  - Client is safe for concurrent use; sub-clients are read-only after
    construction.
  - REST calls go through the spot-bound REST client; it shares the parent's
    signer + rate-limit observers (see kucoin.Client.NewSectionRESTClient).
*/

package account

import (
	kucoin "github.com/tonymontanov/go-kucoin/v2"
	"github.com/tonymontanov/go-kucoin/v2/internal/rest"
)

// Client — KuCoin Account & Funding profile client.
type Client struct {
	parent  *kucoin.Client
	restCli *rest.Client
	baseURL string

	account    *AccountClient
	deposit    *DepositClient
	withdrawal *WithdrawalClient
	transfer   *TransferClient
	fee        *FeeClient
	currency   *CurrencyClient
}

// NewClient creates an Account profile client. Returns nil if parent is nil.
func NewClient(parent *kucoin.Client) *Client {
	if parent == nil {
		return nil
	}
	var base string = kucoin.SpotFamilyBaseURL(parent.Config())
	var c *Client = &Client{
		parent:  parent,
		restCli: parent.NewSectionRESTClient(base),
		baseURL: base,
	}
	c.account = newAccountClient(c)
	c.deposit = newDepositClient(c)
	c.withdrawal = newWithdrawalClient(c)
	c.transfer = newTransferClient(c)
	c.fee = newFeeClient(c)
	c.currency = newCurrencyClient(c)
	return c
}

// Parent returns the root kucoin.Client.
func (c *Client) Parent() *kucoin.Client { return c.parent }

// Account returns the account summary / balances / ledgers sub-client.
func (c *Client) Account() *AccountClient { return c.account }

// Deposit returns the deposit address / history sub-client.
func (c *Client) Deposit() *DepositClient { return c.deposit }

// Withdrawal returns the withdrawal sub-client.
func (c *Client) Withdrawal() *WithdrawalClient { return c.withdrawal }

// Transfer returns the inter-wallet / flex transfer sub-client.
func (c *Client) Transfer() *TransferClient { return c.transfer }

// Fee returns the trade-fee sub-client.
func (c *Client) Fee() *FeeClient { return c.fee }

// Currency returns the currency-directory sub-client.
func (c *Client) Currency() *CurrencyClient { return c.currency }

// Internal shortcuts shared by sub-clients.
func (c *Client) rest() *rest.Client  { return c.restCli }
func (c *Client) signerEnabled() bool { return c.parent.Signer().Enabled() }

// init registers the factory in the root package so kucoin.Client.Account()
// lazily returns *account.Client. A blank import of
// "github.com/tonymontanov/go-kucoin/v2/account" triggers this init.
func init() {
	kucoin.RegisterAccountFactory(func(parent *kucoin.Client) any {
		return NewClient(parent)
	})
}
