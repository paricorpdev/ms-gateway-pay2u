package pay2u

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"paygate/internal/provider"
)

type Provider struct {
	client *Client
}

func NewProvider(client *Client) *Provider {
	return &Provider{
		client: client,
	}
}

func (p *Provider) Name() string {
	return "pay2u"
}

func (p *Provider) CreateBill(ctx context.Context, req *provider.BillRequest) (*provider.BillResult, error) {
	billingURL := p.getBillingURL(req.PaymentMethodCode)

	listenerURL := req.CallbackURL
	if listenerURL == "" && p.client.config.CallbackBaseURL != "" {
		listenerURL = fmt.Sprintf("%s/api/v1/payments/callback/pay2u", p.client.config.CallbackBaseURL)
	}

	submitReq := &SubmitBillRequest{
		MerchantReff:        req.MerchantReff,
		PaymentMethodCode:   req.PaymentMethodCode,
		URLListenerMerchant: listenerURL,
		URLRedirectMerchant: req.RedirectURL,
		BillTitle:           req.BillTitle,
		BillDescription:     req.BillDescription,
		BillCustomerName:    req.CustomerName,
		BillCustomerPhone:   req.CustomerPhone,
		BillCustomerEmail:   req.CustomerEmail,
		BillCurrency:        req.Currency,
		Amount:              req.Amount,
		AmountAdmin:         req.AmountAdmin,
		AmountDiscount:      req.AmountDiscount,
		AmountTotal:         req.AmountTotal,
		ExpiredMinutes:      req.ExpiredMinutes,
	}

	res, err := p.client.SubmitBill(ctx, billingURL, submitReq)
	if err != nil {
		return nil, err
	}

	return &provider.BillResult{
		Token:             res.Token,
		MerchantReff:      res.MerchantReff,
		PaymentMethodCode: res.PaymentMethodCode,
		PaymentCode:       res.PaymentCode,
		AmountTotal:       int64(res.AmountTotal),
		ExpiredMinutes:    res.ExpiredMinutes,
		Status:            res.Status,
	}, nil
}

func (p *Provider) GetBill(ctx context.Context, token, methodCode string) (*provider.BillResult, error) {
	res, err := p.client.GetBill(ctx, token, methodCode)
	if err != nil {
		return nil, err
	}

	return &provider.BillResult{
		Token:             res.Token,
		MerchantReff:      res.MerchantReff,
		PaymentMethodCode: res.PaymentMethodCode,
		PaymentCode:       res.PaymentCode,
		AmountTotal:       int64(res.AmountTotal),
		ExpiredMinutes:    res.ExpiredMinutes,
		Status:            res.Status,
	}, nil
}

func (p *Provider) ParseCallback(payload []byte) (*provider.CallbackResult, error) {
	var cb struct {
		Token             string  `json:"token"`
		MerchantReff      string  `json:"merchant_reff"`
		PaymentMethodCode string  `json:"payment_method_code"`
		PaymentCode       string  `json:"payment_code"`
		PaymentReff       string  `json:"payment_reff"`
		PaymentDate       string  `json:"payment_date"`
		AmountPaid        float64 `json:"amount_paid"`
		Status            int     `json:"status"`
	}

	if err := json.Unmarshal(payload, &cb); err != nil {
		return nil, fmt.Errorf("unmarshal callback payload: %w", err)
	}

	return &provider.CallbackResult{
		MerchantReff:  cb.MerchantReff,
		ProviderToken: cb.Token,
		PaymentReff:   cb.PaymentReff,
		PaymentDate:   cb.PaymentDate,
		AmountPaid:    int64(cb.AmountPaid),
		Status:        cb.Status,
	}, nil
}

func (p *Provider) getBillingURL(paymentMethodCode string) string {
	code := strings.ToUpper(paymentMethodCode)
	if strings.HasPrefix(code, "VA-") || strings.HasPrefix(code, "BRIVA-") || strings.HasPrefix(code, "BNIVA-") {
		return p.client.config.BaseURL + p.client.config.BillingVAURL
	}
	if strings.HasPrefix(code, "QRIS-") {
		return p.client.config.BaseURL + p.client.config.BillingQRISURL
	}
	if strings.HasPrefix(code, "CC-") {
		return p.client.config.BaseURL + p.client.config.BillingCCURL
	}
	return p.client.config.BaseURL + p.client.config.BillingVAURL
}

var _ provider.PaymentProvider = (*Provider)(nil)
