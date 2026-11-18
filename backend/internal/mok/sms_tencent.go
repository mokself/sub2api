package mok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

// SMSSender sends one templated message to one mainland China number (11 digits).
type SMSSender interface {
	Send(ctx context.Context, cfg SMSConfig, phone, templateID string, params []string) (requestID string, err error)
}

// tencentSMSSender calls the SendSms API (version 2021-01-11) through the SDK's common
// client, which upstream already depends on for its captcha; no new module is needed.
type tencentSMSSender struct {
	endpoint string
	scheme   string
}

// NewTencentSMSSender returns the Tencent Cloud SMS sender.
func NewTencentSMSSender() SMSSender {
	return &tencentSMSSender{endpoint: "sms.tencentcloudapi.com", scheme: "HTTPS"}
}

const tencentSMSTimeoutSeconds = 10

type tencentSendSMSResponse struct {
	Response struct {
		SendStatusSet []struct {
			PhoneNumber string `json:"PhoneNumber"`
			Code        string `json:"Code"`
			Message     string `json:"Message"`
		} `json:"SendStatusSet"`
		RequestID string `json:"RequestId"`
	} `json:"Response"`
}

func (s *tencentSMSSender) Send(ctx context.Context, cfg SMSConfig, phone, templateID string, params []string) (string, error) {
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = s.endpoint
	clientProfile.HttpProfile.Scheme = s.scheme
	clientProfile.HttpProfile.ReqMethod = "POST"
	clientProfile.HttpProfile.ReqTimeout = tencentSMSTimeoutSeconds
	client := common.NewCommonClient(common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, clientProfile)

	request := tchttp.NewCommonRequest("sms", "2021-01-11", "SendSms")
	request.SetContext(ctx)
	if params == nil {
		params = []string{}
	}
	if err := request.SetActionParameters(map[string]any{
		"PhoneNumberSet":   []string{"+86" + phone},
		"SmsSdkAppId":      cfg.SDKAppID,
		"SignName":         cfg.SignName,
		"TemplateId":       templateID,
		"TemplateParamSet": params,
	}); err != nil {
		return "", err
	}
	response := tchttp.NewCommonResponse()
	if err := client.Send(request, response); err != nil {
		var sdkErr *tcerr.TencentCloudSDKError
		if errors.As(err, &sdkErr) {
			return sdkErr.GetRequestId(), fmt.Errorf("%s: %s", sdkErr.GetCode(), sdkErr.GetMessage())
		}
		return "", err
	}
	var parsed tencentSendSMSResponse
	if err := json.Unmarshal(response.GetBody(), &parsed); err != nil {
		return "", fmt.Errorf("parse SendSms response: %w", err)
	}
	requestID := parsed.Response.RequestID
	if len(parsed.Response.SendStatusSet) == 0 {
		return requestID, errors.New("SendSms returned no send status")
	}
	status := parsed.Response.SendStatusSet[0]
	if !strings.EqualFold(status.Code, "Ok") {
		return requestID, fmt.Errorf("%s: %s", status.Code, status.Message)
	}
	return requestID, nil
}
