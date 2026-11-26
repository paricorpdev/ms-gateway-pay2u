package pay2u

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"paygate/internal/provider"
)

type bankInquiryData struct {
	Token                 string `json:"token"`
	MerchantReff          string `json:"merchant_reff"`
	ReceiverAccountNumber string `json:"receiver_account_number"`
	ReceiverName          string `json:"receiver_name"`
	ReceiverBank          string `json:"receiver_bank"`
	ReceiverBankName      string `json:"receiver_bank_name"`
	AmountTotal           int64  `json:"amount_total"`
}

type bankTransferData struct {
	Token                 string `json:"token"`
	MerchantReff          string `json:"merchant_reff"`
	ReceiverAccountNumber string `json:"receiver_account_number"`
	ReceiverName          string `json:"receiver_name"`
	ReceiverBank          string `json:"receiver_bank"`
	ReceiverBankName      string `json:"receiver_bank_name"`
	AmountTotal           int64  `json:"amount_total"`
	ReferenceNo           string `json:"reference_no"`
	TransactionDate       string `json:"transaction_date"`
}

func (c *Client) AccountInquiry(ctx context.Context, input *provider.BankTransferRequest) (*provider.BankInquiryResult, error) {
	if input == nil {
		return nil, fmt.Errorf("account inquiry request is required")
	}

	for attempt := 1; attempt <= 2; attempt++ {
		oauthToken, err := c.GetOAuthToken(ctx, attempt > 1)
		if err != nil {
			return nil, err
		}

		transactionToken, err := c.CreateTransactionToken(ctx, oauthToken)
		if err != nil {
			if strings.HasPrefix(err.Error(), "auth_expired:") && attempt < 2 {
				continue
			}
			return nil, err
		}

		request := *input
		request.Token = transactionToken
		if c.config.CallbackBaseURL == "" {
			return nil, fmt.Errorf("pay2u.callback_base_url is required for disbursement callbacks")
		}
		request.URLListenerMerchant = c.disbursementListenerURL()
		response, rawResponse, err := c.postBankRequest(ctx, provider.RequestTypeBankInquiry, c.config.TransferBankInquiryURL, oauthToken, request)
		if err != nil {
			return nil, err
		}
		if (response.RC == "07" || response.RC == "09") && attempt < 2 {
			continue
		}
		if response.RC != "00" {
			return nil, &provider.APIError{Code: response.RC, Description: response.RD}
		}

		var data bankInquiryData
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return nil, fmt.Errorf("decode account inquiry data: %w", err)
		}
		if data.Token == "" {
			data.Token = transactionToken
		}
		if data.MerchantReff == "" {
			data.MerchantReff = input.MerchantReff
		}
		if data.ReceiverAccountNumber == "" {
			data.ReceiverAccountNumber = input.ReceiverAccountNumber
		}
		if data.ReceiverName == "" {
			data.ReceiverName = input.ReceiverName
		}
		if data.ReceiverBank == "" {
			data.ReceiverBank = input.ReceiverBank
		}
		if data.AmountTotal == 0 {
			data.AmountTotal = input.AmountTotal
		}

		return &provider.BankInquiryResult{
			Token:                 data.Token,
			MerchantReff:          data.MerchantReff,
			ReceiverAccountNumber: data.ReceiverAccountNumber,
			ReceiverName:          data.ReceiverName,
			ReceiverBank:          data.ReceiverBank,
			ReceiverBankName:      data.ReceiverBankName,
			AmountTotal:           data.AmountTotal,
			RawResponse:           rawResponse,
		}, nil
	}

	return nil, fmt.Errorf("account inquiry failed after refreshing provider credentials")
}

func (c *Client) AccountTransfer(ctx context.Context, request provider.BankTransferRequest) (*provider.BankTransferResult, error) {
	if request.Token == "" {
		return nil, fmt.Errorf("account transfer token is required")
	}
	oauthToken, err := c.GetOAuthToken(ctx, false)
	if err != nil {
		return nil, err
	}
	if c.config.CallbackBaseURL == "" {
		return nil, fmt.Errorf("pay2u.callback_base_url is required for disbursement callbacks")
	}
	request.URLListenerMerchant = c.disbursementListenerURL()

	response, rawResponse, err := c.postBankRequest(ctx, provider.RequestTypeBankTransfer, c.config.TransferBankURL, oauthToken, request)
	if err != nil {
		return nil, err
	}
	if response.RC != "00" {
		return nil, &provider.APIError{Code: response.RC, Description: response.RD}
	}

	var data bankTransferData
	if err := json.Unmarshal(response.Data, &data); err != nil {
		return nil, &provider.OutcomeUnknownError{Err: fmt.Errorf("decode account transfer data: %w", err)}
	}
	if data.Token == "" {
		data.Token = request.Token
	}
	if data.MerchantReff == "" {
		data.MerchantReff = request.MerchantReff
	}
	if data.ReceiverAccountNumber == "" {
		data.ReceiverAccountNumber = request.ReceiverAccountNumber
	}
	if data.ReceiverName == "" {
		data.ReceiverName = request.ReceiverName
	}
	if data.ReceiverBank == "" {
		data.ReceiverBank = request.ReceiverBank
	}
	if data.AmountTotal == 0 {
		data.AmountTotal = request.AmountTotal
	}

	return &provider.BankTransferResult{
		Token:                 data.Token,
		MerchantReff:          data.MerchantReff,
		ReceiverAccountNumber: data.ReceiverAccountNumber,
		ReceiverName:          data.ReceiverName,
		ReceiverBank:          data.ReceiverBank,
		ReceiverBankName:      data.ReceiverBankName,
		AmountTotal:           data.AmountTotal,
		ReferenceNo:           data.ReferenceNo,
		TransactionDate:       data.TransactionDate,
		RawResponse:           rawResponse,
	}, nil
}

func (c *Client) postBankRequest(ctx context.Context, requestType, endpoint, oauthToken string, body any) (*pay2uResponse, []byte, error) {
	if endpoint == "" {
		return nil, nil, fmt.Errorf("Pay2U endpoint for %s is not configured", requestType)
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("encode %s request: %w", requestType, err)
	}

	reqURL := c.config.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("create %s request: %w", requestType, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("merchant_code", c.config.MerchantCode)
	req.Header.Set("oauth_token", oauthToken)
	if c.config.MerchantCode == "" || oauthToken == "" {
		return nil, nil, fmt.Errorf("Pay2U merchant code and OAuth token are required")
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"merchant_code": c.config.MerchantCode,
		"oauth_token":   oauthToken,
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.recordOutbound(ctx, requestType, 1, reqURL, http.MethodPost, headers, bodyBytes, 0, []byte(err.Error()), latency)
		return nil, nil, &provider.OutcomeUnknownError{Err: fmt.Errorf("send %s request: %w", requestType, err)}
	}
	defer resp.Body.Close()

	responseBytes, _ := io.ReadAll(resp.Body)
	c.recordOutbound(ctx, requestType, 1, reqURL, http.MethodPost, headers, bodyBytes, resp.StatusCode, responseBytes, latency)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseBytes, &provider.OutcomeUnknownError{Err: fmt.Errorf("%s returned HTTP %d: %s", requestType, resp.StatusCode, string(responseBytes))}
	}

	var response pay2uResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, responseBytes, &provider.OutcomeUnknownError{Err: fmt.Errorf("decode %s response: %w", requestType, err)}
	}
	return &response, responseBytes, nil
}

func (c *Client) disbursementListenerURL() string {
	return strings.TrimRight(c.config.CallbackBaseURL, "/") + "/api/v1/disbursements/callback/pay2u"
}

func (p *Provider) AccountInquiry(ctx context.Context, req *provider.BankTransferRequest) (*provider.BankInquiryResult, error) {
	return p.client.AccountInquiry(ctx, req)
}

func (p *Provider) AccountTransfer(ctx context.Context, req *provider.BankTransferRequest) (*provider.BankTransferResult, error) {
	if req == nil {
		return nil, fmt.Errorf("account transfer request is required")
	}
	return p.client.AccountTransfer(ctx, *req)
}

func (p *Provider) ParseTransferCallback(raw []byte) (*provider.BankTransferCallbackResult, error) {
	var callback struct {
		Token                 string  `json:"token"`
		MerchantReff          string  `json:"merchant_reff"`
		ChannelCode           string  `json:"channel_code"`
		ReceiverAccountNumber string  `json:"receiver_account_number"`
		ReceiverName          string  `json:"receiver_name"`
		ReceiverBank          string  `json:"receiver_bank"`
		ReceiverBankName      string  `json:"receiver_bank_name"`
		AmountPaid            float64 `json:"amount_paid"`
		Status                int     `json:"status"`
		TransferDate          string  `json:"transfer_date"`
		ReferenceNo           string  `json:"reference_no"`
	}
	if err := json.Unmarshal(raw, &callback); err != nil {
		return nil, fmt.Errorf("decode account transfer callback: %w", err)
	}
	if callback.Token == "" || callback.MerchantReff == "" {
		return nil, fmt.Errorf("account transfer callback is missing token or merchant_reff")
	}
	return &provider.BankTransferCallbackResult{
		Token:                 callback.Token,
		MerchantReff:          callback.MerchantReff,
		ChannelCode:           callback.ChannelCode,
		ReceiverAccountNumber: callback.ReceiverAccountNumber,
		ReceiverName:          callback.ReceiverName,
		ReceiverBank:          callback.ReceiverBank,
		ReceiverBankName:      callback.ReceiverBankName,
		AmountPaid:            int64(callback.AmountPaid),
		Status:                callback.Status,
		TransferDate:          callback.TransferDate,
		ReferenceNo:           callback.ReferenceNo,
	}, nil
}

var _ provider.DisbursementProvider = (*Provider)(nil)
