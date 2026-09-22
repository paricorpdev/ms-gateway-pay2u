package pay2u

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"paygate/internal/audit"
	"paygate/internal/entity"
	"paygate/internal/logger"
	"paygate/internal/provider"
	"paygate/internal/repository"

	"github.com/google/uuid"
)

type Config struct {
	BaseURL         string
	OAuthURL        string
	TokenURL        string
	BillingVAURL    string
	BillingQRISURL  string
	BillingCCURL    string
	BillingGetURL   string
	ClientID        string
	ClientSecret    string
	MerchantCode    string
	MerchantUser    string
	MerchantPass    string
	MerchantDomain  string
	Timeout         time.Duration
	OAuthTokenTTL   time.Duration
	CallbackBaseURL string
}

type pay2uResponse struct {
	RC   string          `json:"rc"`
	RD   string          `json:"rd"`
	Data json.RawMessage `json:"data,omitempty"`
}

type oauthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	JTI         string `json:"jti"`
}

type tokenData struct {
	Token string `json:"token"`
}

type BillData struct {
	Token               string  `json:"token"`
	BillTitle           string  `json:"bill_title"`
	BillDescription     string  `json:"bill_description"`
	BillCurrency        string  `json:"bill_currency"`
	Amount              float64 `json:"amount"`
	AmountAdmin         float64 `json:"amount_admin"`
	AmountDiscount      float64 `json:"amount_discount"`
	AmountTotal         float64 `json:"amount_total"`
	BillCustomerName    string  `json:"bill_customer_name"`
	BillCustomerPhone   string  `json:"bill_customer_phone"`
	BillCustomerEmail   string  `json:"bill_customer_email"`
	MerchantReff        string  `json:"merchant_reff"`
	ExpiredMinutes      int     `json:"expired_minutes"`
	PaymentMethodCode   string  `json:"payment_method_code"`
	PaymentCode         string  `json:"payment_code"`
	Status              int     `json:"status"`
	URLListenerMerchant string  `json:"url_listener_merchant"`
	URLRedirectMerchant string  `json:"url_redirect_merchant"`
}

type SubmitBillRequest struct {
	Token               string `json:"token"`
	MerchantReff        string `json:"merchant_reff"`
	PaymentMethodCode   string `json:"payment_method_code"`
	URLListenerMerchant string `json:"url_listener_merchant"`
	URLRedirectMerchant string `json:"url_redirect_merchant,omitempty"`
	BillTitle           string `json:"bill_title"`
	BillDescription     string `json:"bill_description,omitempty"`
	BillCustomerName    string `json:"bill_customer_name"`
	BillCustomerPhone   string `json:"bill_customer_phone"`
	BillCustomerEmail   string `json:"bill_customer_email,omitempty"`
	BillCurrency        string `json:"bill_currency"`
	Amount              int64  `json:"amount"`
	AmountAdmin         int64  `json:"amount_admin"`
	AmountDiscount      int64  `json:"amount_discount"`
	AmountTotal         int64  `json:"amount_total"`
	ExpiredMinutes      int    `json:"expired_minutes"`
}

type GetBillRequest struct {
	Token             string `json:"token"`
	PaymentMethodCode string `json:"payment_method_code"`
}

type tokenRequest struct {
	MerchantUser string `json:"merchant_user"`
	MerchantPass string `json:"merchant_pass"`
	Domain       string `json:"domain"`
}

type Client struct {
	config Config
	http   *http.Client
	cache  repository.CacheRepository
	audit  *audit.AuditWorker
}

func NewClient(config Config, cache repository.CacheRepository, auditWorker *audit.AuditWorker) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		config: config,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		cache: cache,
		audit: auditWorker,
	}
}

func (c *Client) recordOutbound(ctx context.Context, reqType string, attempt int, endpoint, method string, headers map[string]string, reqBody []byte, respStatus int, respBody []byte, latency int64) {
	if c.audit == nil {
		return
	}

	reqID := logger.RequestIDFromContext(ctx)
	if reqID == "" {
		reqID = uuid.NewString()
	}

	headersJSON, _ := json.Marshal(headers)

	c.audit.RecordOutbound(&entity.OutboundRequest{
		ID:              uuid.New(),
		RequestID:       reqID,
		Provider:        "pay2u",
		RequestType:     reqType,
		Attempt:         attempt,
		Endpoint:        endpoint,
		Method:          method,
		Headers:         entity.JSONB(headersJSON),
		RequestPayload:  entity.JSONB(reqBody),
		ResponseStatus:  respStatus,
		ResponsePayload: entity.JSONB(respBody),
		LatencyMs:       latency,
		CreatedAt:       time.Now().UTC(),
	})
}

func (c *Client) GetOAuthToken(ctx context.Context, forceRefresh bool) (string, error) {
	cacheKey := "pay2u:oauth_token"
	if !forceRefresh && c.cache != nil {
		if cached, err := c.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
			return string(cached), nil
		}
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.config.ClientID)
	form.Set("client_secret", c.config.ClientSecret)
	formData := form.Encode()

	reqURL := c.config.BaseURL + c.config.OAuthURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("create oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.recordOutbound(ctx, provider.RequestTypeOAuthToken, 1, reqURL, http.MethodPost, headers, []byte(formData), 0, []byte(err.Error()), latency)
		return "", fmt.Errorf("do oauth request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.recordOutbound(ctx, provider.RequestTypeOAuthToken, 1, reqURL, http.MethodPost, headers, []byte(formData), resp.StatusCode, body, latency)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("oauth failed with status %d: %s", resp.StatusCode, string(body))
	}

	var oauth oauthResponse
	if err := json.Unmarshal(body, &oauth); err != nil {
		return "", fmt.Errorf("decode oauth response: %w", err)
	}

	ttl := c.config.OAuthTokenTTL
	if ttl <= 0 {
		ttl = 50 * time.Minute
	}

	if c.cache != nil {
		_ = c.cache.Set(ctx, cacheKey, []byte(oauth.AccessToken), ttl)
	}

	return oauth.AccessToken, nil
}

func (c *Client) CreateTransactionToken(ctx context.Context, oauthToken string) (string, error) {
	reqBody := tokenRequest{
		MerchantUser: c.config.MerchantUser,
		MerchantPass: c.config.MerchantPass,
		Domain:       c.config.MerchantDomain,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	reqURL := c.config.BaseURL + c.config.TokenURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("merchant_code", c.config.MerchantCode)
	req.Header.Set("oauth_token", oauthToken)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"merchant_code": c.config.MerchantCode,
		"oauth_token":   oauthToken,
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.recordOutbound(ctx, provider.RequestTypeTxToken, 1, reqURL, http.MethodPost, headers, bodyBytes, 0, []byte(err.Error()), latency)
		return "", fmt.Errorf("do token request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	c.recordOutbound(ctx, provider.RequestTypeTxToken, 1, reqURL, http.MethodPost, headers, bodyBytes, resp.StatusCode, respBytes, latency)

	var pResp pay2uResponse
	if err := json.Unmarshal(respBytes, &pResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if pResp.RC == "07" || pResp.RC == "09" {
		return "", fmt.Errorf("auth_expired: %s", pResp.RC)
	}
	if pResp.RC != "00" {
		return "", fmt.Errorf("token create error %s: %s", pResp.RC, pResp.RD)
	}

	var data tokenData
	if err := json.Unmarshal(pResp.Data, &data); err != nil {
		return "", fmt.Errorf("unmarshal token data: %w", err)
	}

	return data.Token, nil
}

func (c *Client) SubmitBill(ctx context.Context, billingEndpoint string, req *SubmitBillRequest) (*BillData, error) {
	return c.submitBillWithRetry(ctx, billingEndpoint, req, 1)
}

func (c *Client) submitBillWithRetry(ctx context.Context, billingEndpoint string, req *SubmitBillRequest, attempt int) (*BillData, error) {
	oauthToken, err := c.GetOAuthToken(ctx, false)
	if err != nil {
		return nil, err
	}

	txToken, err := c.CreateTransactionToken(ctx, oauthToken)
	if err != nil {
		if strings.HasPrefix(err.Error(), "auth_expired:") && attempt < 2 {
			oauthToken, err = c.GetOAuthToken(ctx, true)
			if err != nil {
				return nil, err
			}
			txToken, err = c.CreateTransactionToken(ctx, oauthToken)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	req.Token = txToken

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, billingEndpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("merchant_code", c.config.MerchantCode)
	httpReq.Header.Set("oauth_token", oauthToken)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"merchant_code": c.config.MerchantCode,
		"oauth_token":   oauthToken,
	}

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.recordOutbound(ctx, provider.RequestTypeSubmitBill, attempt, billingEndpoint, http.MethodPost, headers, bodyBytes, 0, []byte(err.Error()), latency)
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	c.recordOutbound(ctx, provider.RequestTypeSubmitBill, attempt, billingEndpoint, http.MethodPost, headers, bodyBytes, resp.StatusCode, respBytes, latency)

	var pResp pay2uResponse
	if err := json.Unmarshal(respBytes, &pResp); err != nil {
		return nil, fmt.Errorf("decode bill response: %w", err)
	}

	if (pResp.RC == "07" || pResp.RC == "09") && attempt < 2 {
		_, _ = c.GetOAuthToken(ctx, true)
		return c.submitBillWithRetry(ctx, billingEndpoint, req, attempt+1)
	}

	if pResp.RC != "00" {
		return nil, fmt.Errorf("submit bill error %s: %s", pResp.RC, pResp.RD)
	}

	var data BillData
	if err := json.Unmarshal(pResp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal bill data: %w", err)
	}

	return &data, nil
}

func (c *Client) GetBill(ctx context.Context, token, paymentMethodCode string) (*BillData, error) {
	return c.getBillWithRetry(ctx, token, paymentMethodCode, 1)
}

func (c *Client) getBillWithRetry(ctx context.Context, token, paymentMethodCode string, attempt int) (*BillData, error) {
	oauthToken, err := c.GetOAuthToken(ctx, false)
	if err != nil {
		return nil, err
	}

	req := GetBillRequest{
		Token:             token,
		PaymentMethodCode: paymentMethodCode,
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqURL := c.config.BaseURL + c.config.BillingGetURL
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("merchant_code", c.config.MerchantCode)
	httpReq.Header.Set("oauth_token", oauthToken)

	headers := map[string]string{
		"Content-Type":  "application/json",
		"merchant_code": c.config.MerchantCode,
		"oauth_token":   oauthToken,
	}

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.recordOutbound(ctx, provider.RequestTypeGetBill, attempt, reqURL, http.MethodPost, headers, bodyBytes, 0, []byte(err.Error()), latency)
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	c.recordOutbound(ctx, provider.RequestTypeGetBill, attempt, reqURL, http.MethodPost, headers, bodyBytes, resp.StatusCode, respBytes, latency)

	var pResp pay2uResponse
	if err := json.Unmarshal(respBytes, &pResp); err != nil {
		return nil, fmt.Errorf("decode get bill response: %w", err)
	}

	if (pResp.RC == "07" || pResp.RC == "09") && attempt < 2 {
		_, _ = c.GetOAuthToken(ctx, true)
		return c.getBillWithRetry(ctx, token, paymentMethodCode, attempt+1)
	}

	if pResp.RC != "00" {
		return nil, fmt.Errorf("get bill error %s: %s", pResp.RC, pResp.RD)
	}

	var data BillData
	if err := json.Unmarshal(pResp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal bill data: %w", err)
	}

	return &data, nil
}
