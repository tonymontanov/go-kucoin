/*
FILE: account/contract_rest_test.go

DESCRIPTION:
Offline contract tests for the Account & Funding REST surface. A mock HTTP
server returns recorded KuCoin response bodies (wrapped in the { code, data }
envelope); the tests drive the real sub-clients end-to-end through the SDK
transport and assert the typed output — exercising path construction, envelope
parsing, the wire→type converters and request-body assembly (withdraw / flex
transfer).

No network access: an explicit Config.REST.BaseURL (the httptest URL) is a
non-futures host, so the account profile honours it as-is.
*/

package account

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	kucoin "github.com/tonymontanov/go-kucoin/v2"
	accounttypes "github.com/tonymontanov/go-kucoin/v2/account/types"
)

func requireDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func writeEnv(w http.ResponseWriter, dataJSON string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"code":"200000","data":`+dataJSON+`}`)
}

type recordedBodies struct {
	mu   sync.Mutex
	body map[string]string
}

func (r *recordedBodies) set(path, body string) {
	r.mu.Lock()
	r.body[path] = body
	r.mu.Unlock()
}

func (r *recordedBodies) get(path string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body[path]
}

const (
	fixtureSummary  = `{"level":2,"subQuantity":3,"spotSubQuantity":3,"marginSubQuantity":2,"futuresSubQuantity":2,"optionSubQuantity":0,"maxSubQuantity":5,"maxDefaultSubQuantity":5,"maxSpotSubQuantity":0,"maxMarginSubQuantity":0,"maxFuturesSubQuantity":0,"maxOptionSubQuantity":0}`
	fixtureAPIKey   = `{"remark":"No ip","apiKey":"abc***def","apiVersion":3,"permission":"General,Spot,Margin","ipWhitelist":"","createdAt":1758765668000,"uid":165215,"isMaster":true}`
	fixtureAccount  = `{"id":"5bd6e9286d99522a52e458de","currency":"USDT","type":"trade","balance":"100.5","available":"90.2","holds":"10.3"}`
	fixtureLedger   = `{"id":"265329987780896","currency":"USDT","amount":"0.01","fee":"0","balance":"0","accountType":"TRADE","bizType":"SUB_TRANSFER","direction":"out","createdAt":1728658481484,"context":""}`
	fixtureDepAddr  = `{"address":"0xdeadbeef","memo":"","remark":null,"chainId":"trx","to":"TRADE","expirationDate":0,"currency":"USDT","chainName":"TRC20"}`
	fixtureDeposit  = `{"currency":"USDT","chain":"trx","amount":"50","fee":"0","walletTxId":"tx-1","address":"0xabc","memo":"","isInner":false,"status":"SUCCESS","remark":"","createdAt":1700000000000,"updatedAt":1700000001000}`
	fixtureWdQuota  = `{"currency":"USDT","limitBTCAmount":"15.79","usedBTCAmount":"0","remainAmount":"15.79","availableAmount":"500","withdrawMinFee":"1","withdrawMinSize":"5","isWithdrawEnabled":true,"precision":4,"chain":"trx"}`
	fixtureWdRecord = `{"id":"wd-1","currency":"USDT","chain":"trx","amount":"10","fee":"1","address":"0xabc","memo":"","walletTxId":"tx-9","isInner":false,"status":"SUCCESS","remark":"","createdAt":1700000000000,"updatedAt":1700000002000}`
	fixtureXfer     = `{"currency":"USDT","balance":"10.5","available":"10.5","holds":"0","transferable":"10.5"}`
	fixtureCurrency = `{"currency":"USDT","name":"USDT","fullName":"Tether","precision":8,"confirms":null,"contractAddress":null,"isMarginEnabled":true,"isDebitEnabled":true,"chains":[{"chainName":"TRC20","withdrawalMinSize":"5","depositMinSize":null,"withdrawFeeRate":"0","withdrawalMinFee":"1","isWithdrawEnabled":true,"isDepositEnabled":true,"confirms":1,"preConfirms":1,"contractAddress":"T...","withdrawPrecision":6,"maxWithdraw":null,"maxDeposit":null,"needTag":false,"chainId":"trx"}]}`
)

func newMockRESTClient(t *testing.T, withCreds bool) (*Client, *recordedBodies) {
	t.Helper()
	var rec = &recordedBodies{body: map[string]string{}}
	var mux = http.NewServeMux()

	mux.HandleFunc("/api/v2/user-info", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureSummary)
	})
	mux.HandleFunc("/api/v1/user/api-key", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureAPIKey)
	})
	// /api/v1/accounts (list) and the subtree (/ledgers, /transferable, /{id}).
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureAccount+`]`)
	})
	mux.HandleFunc("/api/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ledgers"):
			writeEnv(w, `{"currentPage":1,"pageSize":50,"totalNum":1,"totalPage":1,"items":[`+fixtureLedger+`]}`)
		case strings.HasSuffix(r.URL.Path, "/transferable"):
			writeEnv(w, fixtureXfer)
		default:
			writeEnv(w, fixtureAccount)
		}
	})

	mux.HandleFunc("/api/v3/deposit-address/create", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("/api/v3/deposit-address/create", string(b))
		writeEnv(w, fixtureDepAddr)
	})
	mux.HandleFunc("/api/v3/deposit-addresses", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureDepAddr+`]`)
	})
	mux.HandleFunc("/api/v1/deposits", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"currentPage":1,"pageSize":50,"totalNum":1,"totalPage":1,"items":[`+fixtureDeposit+`]}`)
	})

	mux.HandleFunc("/api/v3/withdrawals", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("/api/v3/withdrawals", string(b))
		writeEnv(w, `{"withdrawalId":"670deec84d64da0007d7c946"}`)
	})
	mux.HandleFunc("/api/v1/withdrawals", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"currentPage":1,"pageSize":50,"totalNum":1,"totalPage":1,"items":[`+fixtureWdRecord+`]}`)
	})
	mux.HandleFunc("/api/v1/withdrawals/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/quotas"):
			writeEnv(w, fixtureWdQuota)
		case r.Method == http.MethodDelete:
			writeEnv(w, `null`)
		default:
			writeEnv(w, fixtureWdRecord)
		}
	})

	mux.HandleFunc("/api/v3/accounts/universal-transfer", func(w http.ResponseWriter, r *http.Request) {
		var b, _ = io.ReadAll(r.Body)
		rec.set("/api/v3/accounts/universal-transfer", string(b))
		writeEnv(w, `{"orderId":"6705f7248c6954000733ecac"}`)
	})

	mux.HandleFunc("/api/v1/base-fee", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `{"takerFeeRate":"0.001","makerFeeRate":"0.0008"}`)
	})
	mux.HandleFunc("/api/v1/trade-fees", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[{"symbol":"BTC-USDT","takerFeeRate":"0.001","makerFeeRate":"0.001"}]`)
	})

	mux.HandleFunc("/api/v3/currencies", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, `[`+fixtureCurrency+`]`)
	})
	mux.HandleFunc("/api/v3/currencies/", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, fixtureCurrency)
	})

	var srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var cfg = kucoin.DefaultConfig()
	cfg.REST.BaseURL = srv.URL
	if withCreds {
		cfg.APIKey = "k"
		cfg.SecretKey = "s"
		cfg.Passphrase = "p"
	}
	var parent, err = kucoin.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewClient(parent), rec
}

func TestContractREST_GetSummary(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var s, err = c.Account().GetSummary(context.Background())
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if s.Level != 2 || s.SubQuantity != 3 || s.MaxSubQuantity != 5 {
		t.Errorf("summary = %+v", s)
	}
}

func TestContractREST_GetApiKeyInfo(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var info, err = c.Account().GetApiKeyInfo(context.Background())
	if err != nil {
		t.Fatalf("GetApiKeyInfo: %v", err)
	}
	if info.UID != "165215" || !info.IsMaster || info.APIVersion != 3 {
		t.Errorf("apikey = %+v", info)
	}
}

func TestContractREST_GetAccounts(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var got, err = c.Account().GetAccounts(context.Background(), "USDT", accounttypes.AccountTrade)
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(got) != 1 || got[0].Currency != "USDT" || got[0].Available.String() != "90.2" {
		t.Fatalf("accounts = %+v", got)
	}
}

func TestContractREST_GetAccount_BackfillsID(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var got, err = c.Account().GetAccount(context.Background(), "5bd6e9286d99522a52e458de")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.ID != "5bd6e9286d99522a52e458de" || got.Balance.String() != "100.5" {
		t.Errorf("account = %+v", got)
	}
}

func TestContractREST_GetLedgers(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var page, err = c.Account().GetLedgers(context.Background(), accounttypes.LedgerQuery{Currency: "USDT", Direction: accounttypes.DirectionOut})
	if err != nil {
		t.Fatalf("GetLedgers: %v", err)
	}
	if page.TotalNum != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].Direction != accounttypes.DirectionOut || page.Items[0].BizType != "SUB_TRANSFER" {
		t.Errorf("entry = %+v", page.Items[0])
	}
}

func TestContractREST_CreateDepositAddress(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var addr, err = c.Deposit().CreateAddress(context.Background(), "USDT", "trx", "trade")
	if err != nil {
		t.Fatalf("CreateAddress: %v", err)
	}
	if addr.Address != "0xdeadbeef" || addr.ChainName != "TRC20" {
		t.Errorf("addr = %+v", addr)
	}
	var body = rec.get("/api/v3/deposit-address/create")
	if !strings.Contains(body, `"currency":"USDT"`) || !strings.Contains(body, `"chain":"trx"`) {
		t.Errorf("body = %s", body)
	}
}

func TestContractREST_GetDepositHistory(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var page, err = c.Deposit().GetHistory(context.Background(), accounttypes.DepositHistoryQuery{Currency: "USDT"})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != "SUCCESS" || page.Items[0].Amount.String() != "50" {
		t.Errorf("page = %+v", page)
	}
}

func TestContractREST_GetWithdrawalQuotas(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var q, err = c.Withdrawal().GetQuotas(context.Background(), "USDT", "trx")
	if err != nil {
		t.Fatalf("GetQuotas: %v", err)
	}
	if !q.IsWithdrawEnabled || q.WithdrawMinFee.String() != "1" || q.Precision != 4 {
		t.Errorf("quota = %+v", q)
	}
}

func TestContractREST_Withdraw(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Withdrawal().Withdraw(context.Background(), accounttypes.WithdrawRequest{
		Currency: "USDT", ToAddress: "0xabc", Amount: requireDec("10"), Chain: "trx", IsInner: true,
	})
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if res.WithdrawalID != "670deec84d64da0007d7c946" {
		t.Errorf("res = %+v", res)
	}
	var body = rec.get("/api/v3/withdrawals")
	if !strings.Contains(body, `"withdrawType":"ADDRESS"`) || !strings.Contains(body, `"isInner":true`) || !strings.Contains(body, `"amount":"10"`) {
		t.Errorf("body = %s", body)
	}
}

func TestContractREST_Withdraw_Validation(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var _, err = c.Withdrawal().Withdraw(context.Background(), accounttypes.WithdrawRequest{Currency: "USDT", Amount: requireDec("1")})
	if err == nil || !kucoin.IsInvalidRequest(err) {
		t.Fatalf("expected invalid-request, got %v", err)
	}
}

func TestContractREST_GetTransferable(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var b, err = c.Transfer().GetTransferable(context.Background(), "USDT", accounttypes.AccountMain, "")
	if err != nil {
		t.Fatalf("GetTransferable: %v", err)
	}
	if b.Transferable.String() != "10.5" {
		t.Errorf("balance = %+v", b)
	}
}

func TestContractREST_FlexTransfer(t *testing.T) {
	var c, rec = newMockRESTClient(t, true)
	var res, err = c.Transfer().InnerTransfer(context.Background(), "USDT", requireDec("3"), accounttypes.AccountMain, accounttypes.AccountTrade)
	if err != nil {
		t.Fatalf("InnerTransfer: %v", err)
	}
	if res.OrderID != "6705f7248c6954000733ecac" {
		t.Errorf("res = %+v", res)
	}
	var body = rec.get("/api/v3/accounts/universal-transfer")
	if !strings.Contains(body, `"type":"INTERNAL"`) || !strings.Contains(body, `"fromAccountType":"MAIN"`) || !strings.Contains(body, `"toAccountType":"TRADE"`) {
		t.Errorf("body = %s", body)
	}
	if !strings.Contains(body, `"clientOid":"kca-`) {
		t.Errorf("body missing generated clientOid: %s", body)
	}
}

func TestContractREST_GetBaseFee(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var fee, err = c.Fee().GetBaseFee(context.Background(), -1)
	if err != nil {
		t.Fatalf("GetBaseFee: %v", err)
	}
	if fee.MakerFeeRate.String() != "0.0008" || fee.TakerFeeRate.String() != "0.001" {
		t.Errorf("fee = %+v", fee)
	}
}

func TestContractREST_GetTradeFees(t *testing.T) {
	var c, _ = newMockRESTClient(t, true)
	var fees, err = c.Fee().GetTradeFees(context.Background(), []string{"BTC-USDT"})
	if err != nil {
		t.Fatalf("GetTradeFees: %v", err)
	}
	if len(fees) != 1 || fees[0].Symbol != "BTC-USDT" {
		t.Errorf("fees = %+v", fees)
	}
}

func TestContractREST_GetCurrencies(t *testing.T) {
	var c, _ = newMockRESTClient(t, false)
	var cur, err = c.Currency().GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(cur) != 1 || cur[0].Currency != "USDT" || len(cur[0].Chains) != 1 {
		t.Fatalf("currencies = %+v", cur)
	}
	// depositMinSize/maxWithdraw are null on the wire → zero decimals.
	if !cur[0].Chains[0].DepositMinSize.IsZero() || !cur[0].Chains[0].MaxWithdraw.IsZero() {
		t.Errorf("null decimals not zeroed: %+v", cur[0].Chains[0])
	}
	if cur[0].Chains[0].ChainID != "trx" || cur[0].Chains[0].WithdrawalMinFee.String() != "1" {
		t.Errorf("chain = %+v", cur[0].Chains[0])
	}
}

func TestContractREST_AuthRequired(t *testing.T) {
	var c, _ = newMockRESTClient(t, false) // no creds
	var _, err = c.Account().GetSummary(context.Background())
	if err == nil || !kucoin.IsAuth(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
