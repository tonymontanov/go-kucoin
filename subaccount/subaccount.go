/*
FILE: subaccount/subaccount.go

DESCRIPTION:
Master-account sub-account management: create + permission grants, summary /
balance listings, and the spot API-key lifecycle.

ENDPOINTS:
  - POST   /api/v2/sub/user/created          Create
  - POST   /api/v3/sub/user/margin/enable    EnableMargin
  - POST   /api/v3/sub/user/futures/enable   EnableFutures
  - GET    /api/v2/sub/user                  GetSummaries (paged)
  - GET    /api/v1/sub-accounts/{id}         GetBalance
  - GET    /api/v2/sub-accounts              GetBalances (paged)
  - POST   /api/v1/sub/api-key               CreateAPIKey
  - GET    /api/v1/sub/api-key               GetAPIKeys
  - POST   /api/v1/sub/api-key/update        UpdateAPIKey
  - DELETE /api/v1/sub/api-key               DeleteAPIKey
*/

package subaccount

import (
	"context"

	"github.com/shopspring/decimal"

	subtypes "github.com/tonymontanov/go-kucoin/v2/subaccount/types"
)

// Create creates a new sub-account.
func (c *Client) Create(ctx context.Context, req subtypes.CreateRequest) (*subtypes.CreateResult, error) {
	if req.SubName == "" || req.Password == "" || req.Access == "" {
		return nil, errInvalidRequest("Create", "subName, password and access are required")
	}
	var body map[string]any = map[string]any{
		"subName":  req.SubName,
		"password": req.Password,
		"access":   req.Access,
	}
	if req.Remarks != "" {
		body["remarks"] = req.Remarks
	}
	var wire createResultWire
	if err := c.doPOST(ctx, "/api/v2/sub/user/created", body, &wire); err != nil {
		return nil, err
	}
	return &subtypes.CreateResult{UID: wire.UID, SubName: wire.SubName, Remarks: wire.Remarks, Access: wire.Access}, nil
}

// EnableMargin grants margin-trading permission to a sub-account by uid.
func (c *Client) EnableMargin(ctx context.Context, uid string) error {
	if uid == "" {
		return errInvalidRequest("EnableMargin", "uid is required")
	}
	return c.doPOST(ctx, "/api/v3/sub/user/margin/enable", map[string]any{"uid": uid}, nil)
}

// EnableFutures grants futures-trading permission to a sub-account by uid.
func (c *Client) EnableFutures(ctx context.Context, uid string) error {
	if uid == "" {
		return errInvalidRequest("EnableFutures", "uid is required")
	}
	return c.doPOST(ctx, "/api/v3/sub/user/futures/enable", map[string]any{"uid": uid}, nil)
}

// GetSummaries returns the sub-account summary list (paged).
func (c *Client) GetSummaries(ctx context.Context, currentPage, pageSize int) (*subtypes.SubUserPage, error) {
	var query map[string]string = map[string]string{}
	if currentPage > 0 {
		query["currentPage"] = itoa(currentPage)
	}
	if pageSize > 0 {
		query["pageSize"] = itoa(pageSize)
	}
	var wire subUserPageWire
	if err := c.doGET(ctx, "/api/v2/sub/user", query, &wire); err != nil {
		return nil, err
	}
	return wire.toPage(), nil
}

// GetBalance returns a single sub-account's spot/margin balances.
// includeBaseAmount adds base-currency valuations.
func (c *Client) GetBalance(ctx context.Context, subUserID string, includeBaseAmount bool) (*subtypes.SubAccountAssets, error) {
	if subUserID == "" {
		return nil, errInvalidRequest("GetBalance", "subUserID is required")
	}
	var query map[string]string
	if includeBaseAmount {
		query = map[string]string{"includeBaseAmount": "true"}
	}
	var wire subAssetsWire
	if err := c.doGET(ctx, "/api/v1/sub-accounts/"+subUserID, query, &wire); err != nil {
		return nil, err
	}
	var a subtypes.SubAccountAssets = wire.toAssets()
	return &a, nil
}

// GetBalances returns all sub-accounts' spot balances (paged).
func (c *Client) GetBalances(ctx context.Context, currentPage, pageSize int) (*subtypes.SubAccountAssetsPage, error) {
	var query map[string]string = map[string]string{}
	if currentPage > 0 {
		query["currentPage"] = itoa(currentPage)
	}
	if pageSize > 0 {
		query["pageSize"] = itoa(pageSize)
	}
	var wire subAssetsPageWire
	if err := c.doGET(ctx, "/api/v2/sub-accounts", query, &wire); err != nil {
		return nil, err
	}
	return wire.toPage(), nil
}

// CreateAPIKey creates a spot API key for a sub-account. The returned secret and
// passphrase are shown ONCE — persist them immediately.
func (c *Client) CreateAPIKey(ctx context.Context, req subtypes.CreateAPIKeyRequest) (*subtypes.CreatedAPIKey, error) {
	if req.SubName == "" || req.Passphrase == "" || req.Remark == "" {
		return nil, errInvalidRequest("CreateAPIKey", "subName, passphrase and remark are required")
	}
	var body map[string]any = map[string]any{
		"subName":    req.SubName,
		"passphrase": req.Passphrase,
		"remark":     req.Remark,
	}
	if req.Permission != "" {
		body["permission"] = req.Permission
	}
	if req.IPWhitelist != "" {
		body["ipWhitelist"] = req.IPWhitelist
	}
	if req.Expire != "" {
		body["expire"] = req.Expire
	}
	var wire createdAPIKeyWire
	if err := c.doPOST(ctx, "/api/v1/sub/api-key", body, &wire); err != nil {
		return nil, err
	}
	return &subtypes.CreatedAPIKey{
		SubName:     wire.SubName,
		APIKey:      wire.APIKey,
		APISecret:   wire.APISecret,
		Passphrase:  wire.Passphrase,
		Permission:  wire.Permission,
		IPWhitelist: wire.IPWhitelist,
		Remark:      wire.Remark,
		CreatedAt:   int64(wire.CreatedAt),
	}, nil
}

// GetAPIKeys lists the API keys of a sub-account. apiKey optionally narrows to
// one key.
func (c *Client) GetAPIKeys(ctx context.Context, subName, apiKey string) ([]subtypes.APIKey, error) {
	if subName == "" {
		return nil, errInvalidRequest("GetAPIKeys", "subName is required")
	}
	var query map[string]string = map[string]string{"subName": subName}
	if apiKey != "" {
		query["apiKey"] = apiKey
	}
	var wire []apiKeyWire
	if err := c.doGET(ctx, "/api/v1/sub/api-key", query, &wire); err != nil {
		return nil, err
	}
	var out []subtypes.APIKey = make([]subtypes.APIKey, len(wire))
	var i int
	for i = 0; i < len(wire); i++ {
		out[i] = wire[i].toAPIKey()
	}
	return out, nil
}

// UpdateAPIKey modifies a sub-account API key's permission / IP allowlist /
// expiry.
func (c *Client) UpdateAPIKey(ctx context.Context, req subtypes.UpdateAPIKeyRequest) (*subtypes.UpdatedAPIKey, error) {
	if req.SubName == "" || req.APIKey == "" || req.Passphrase == "" {
		return nil, errInvalidRequest("UpdateAPIKey", "subName, apiKey and passphrase are required")
	}
	var body map[string]any = map[string]any{
		"subName":    req.SubName,
		"apiKey":     req.APIKey,
		"passphrase": req.Passphrase,
	}
	if req.Permission != "" {
		body["permission"] = req.Permission
	}
	if req.IPWhitelist != "" {
		body["ipWhitelist"] = req.IPWhitelist
	}
	if req.Expire != "" {
		body["expire"] = req.Expire
	}
	var wire updatedAPIKeyWire
	if err := c.doPOST(ctx, "/api/v1/sub/api-key/update", body, &wire); err != nil {
		return nil, err
	}
	return &subtypes.UpdatedAPIKey{
		SubName:     wire.SubName,
		APIKey:      wire.APIKey,
		Permission:  wire.Permission,
		IPWhitelist: wire.IPWhitelist,
	}, nil
}

// DeleteAPIKey deletes a sub-account API key.
func (c *Client) DeleteAPIKey(ctx context.Context, subName, passphrase, apiKey string) (*subtypes.DeletedAPIKey, error) {
	if subName == "" || passphrase == "" || apiKey == "" {
		return nil, errInvalidRequest("DeleteAPIKey", "subName, passphrase and apiKey are required")
	}
	var query map[string]string = map[string]string{
		"subName":    subName,
		"passphrase": passphrase,
		"apiKey":     apiKey,
	}
	var wire deletedAPIKeyWire
	if err := c.doDELETE(ctx, "/api/v1/sub/api-key", query, &wire); err != nil {
		return nil, err
	}
	return &subtypes.DeletedAPIKey{SubName: wire.SubName, APIKey: wire.APIKey}, nil
}

// ---------------------------------------------------------------------
// Wire structs + converters.
// ---------------------------------------------------------------------

type createResultWire struct {
	UID     int64  `json:"uid"`
	SubName string `json:"subName"`
	Remarks string `json:"remarks"`
	Access  string `json:"access"`
}

type subUserWire struct {
	UserID    string    `json:"userId"`
	UID       int64     `json:"uid"`
	SubName   string    `json:"subName"`
	Status    int       `json:"status"`
	Type      int       `json:"type"`
	Access    string    `json:"access"`
	Remarks   string    `json:"remarks"`
	CreatedAt flexInt64 `json:"createdAt"`
}

func (w subUserWire) toSubUser() subtypes.SubUser {
	return subtypes.SubUser{
		UserID:    w.UserID,
		UID:       w.UID,
		SubName:   w.SubName,
		Status:    w.Status,
		Type:      w.Type,
		Access:    w.Access,
		Remarks:   w.Remarks,
		CreatedAt: int64(w.CreatedAt),
	}
}

type subUserPageWire struct {
	CurrentPage int           `json:"currentPage"`
	PageSize    int           `json:"pageSize"`
	TotalNum    int           `json:"totalNum"`
	TotalPage   int           `json:"totalPage"`
	Items       []subUserWire `json:"items"`
}

func (w subUserPageWire) toPage() *subtypes.SubUserPage {
	var items []subtypes.SubUser = make([]subtypes.SubUser, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toSubUser()
	}
	return &subtypes.SubUserPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type subBalanceWire struct {
	Currency          string          `json:"currency"`
	Balance           decimal.Decimal `json:"balance"`
	Available         decimal.Decimal `json:"available"`
	Holds             decimal.Decimal `json:"holds"`
	BaseCurrency      string          `json:"baseCurrency"`
	BaseCurrencyPrice decimal.Decimal `json:"baseCurrencyPrice"`
	BaseAmount        decimal.Decimal `json:"baseAmount"`
}

func (w subBalanceWire) toBalance() subtypes.SubBalance {
	return subtypes.SubBalance{
		Currency:          w.Currency,
		Balance:           w.Balance,
		Available:         w.Available,
		Holds:             w.Holds,
		BaseCurrency:      w.BaseCurrency,
		BaseCurrencyPrice: w.BaseCurrencyPrice,
		BaseAmount:        w.BaseAmount,
	}
}

func convBalances(in []subBalanceWire) []subtypes.SubBalance {
	var out []subtypes.SubBalance = make([]subtypes.SubBalance, len(in))
	var i int
	for i = 0; i < len(in); i++ {
		out[i] = in[i].toBalance()
	}
	return out
}

type subAssetsWire struct {
	SubUserID      string           `json:"subUserId"`
	SubName        string           `json:"subName"`
	MainAccounts   []subBalanceWire `json:"mainAccounts"`
	TradeAccounts  []subBalanceWire `json:"tradeAccounts"`
	MarginAccounts []subBalanceWire `json:"marginAccounts"`
}

func (w subAssetsWire) toAssets() subtypes.SubAccountAssets {
	return subtypes.SubAccountAssets{
		SubUserID:      w.SubUserID,
		SubName:        w.SubName,
		MainAccounts:   convBalances(w.MainAccounts),
		TradeAccounts:  convBalances(w.TradeAccounts),
		MarginAccounts: convBalances(w.MarginAccounts),
	}
}

type subAssetsPageWire struct {
	CurrentPage int             `json:"currentPage"`
	PageSize    int             `json:"pageSize"`
	TotalNum    int             `json:"totalNum"`
	TotalPage   int             `json:"totalPage"`
	Items       []subAssetsWire `json:"items"`
}

func (w subAssetsPageWire) toPage() *subtypes.SubAccountAssetsPage {
	var items []subtypes.SubAccountAssets = make([]subtypes.SubAccountAssets, len(w.Items))
	var i int
	for i = 0; i < len(w.Items); i++ {
		items[i] = w.Items[i].toAssets()
	}
	return &subtypes.SubAccountAssetsPage{
		CurrentPage: w.CurrentPage,
		PageSize:    w.PageSize,
		TotalNum:    w.TotalNum,
		TotalPage:   w.TotalPage,
		Items:       items,
	}
}

type createdAPIKeyWire struct {
	SubName     string    `json:"subName"`
	APIKey      string    `json:"apiKey"`
	APISecret   string    `json:"apiSecret"`
	Passphrase  string    `json:"passphrase"`
	Permission  string    `json:"permission"`
	IPWhitelist string    `json:"ipWhitelist"`
	Remark      string    `json:"remark"`
	CreatedAt   flexInt64 `json:"createdAt"`
}

type apiKeyWire struct {
	SubName     string    `json:"subName"`
	APIKey      string    `json:"apiKey"`
	Remark      string    `json:"remark"`
	Permission  string    `json:"permission"`
	IPWhitelist string    `json:"ipWhitelist"`
	CreatedAt   flexInt64 `json:"createdAt"`
}

func (w apiKeyWire) toAPIKey() subtypes.APIKey {
	return subtypes.APIKey{
		SubName:     w.SubName,
		APIKey:      w.APIKey,
		Remark:      w.Remark,
		Permission:  w.Permission,
		IPWhitelist: w.IPWhitelist,
		CreatedAt:   int64(w.CreatedAt),
	}
}

type updatedAPIKeyWire struct {
	SubName     string `json:"subName"`
	APIKey      string `json:"apiKey"`
	Permission  string `json:"permission"`
	IPWhitelist string `json:"ipWhitelist"`
}

type deletedAPIKeyWire struct {
	SubName string `json:"subName"`
	APIKey  string `json:"apiKey"`
}
