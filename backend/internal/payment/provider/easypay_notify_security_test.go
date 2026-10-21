package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayVerifyNotificationRejectsOrderCreationReplay(t *testing.T) {
	pkey := "merchant-secret"
	order := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "order-1",
		"notify_url":   "https://site.example/api/v1/payment/webhook/easypay",
		"return_url":   "https://site.example/payment/result?trade_status=TRADE_SUCCESS",
		"name":         "recharge",
		"money":        "10.00",
	}
	order["sign"] = easyPaySign(order, pkey)
	order["sign_type"] = "MD5"
	body := url.Values{}
	for k, v := range order {
		body.Set(k, v)
	}
	prov := &EasyPay{config: map[string]string{"pid": "1001", "pkey": pkey}}
	if _, err := prov.VerifyNotification(context.Background(), body.Encode(), nil); err == nil {
		t.Fatal("order-creation payload must not verify as a payment notification")
	}
}

func TestEasyPayVerifyNotificationAcceptsGenuineNotify(t *testing.T) {
	pkey := "merchant-secret"
	params := map[string]string{
		"pid":          "1001",
		"trade_no":     "upstream-1",
		"out_trade_no": "order-1",
		"type":         "alipay",
		"name":         "recharge",
		"money":        "10.00",
		"trade_status": tradeStatusSuccess,
		"sign_type":    "MD5",
	}
	params["sign"] = easyPaySign(params, pkey)
	body := url.Values{}
	for k, v := range params {
		body.Set(k, v)
	}
	prov := &EasyPay{config: map[string]string{"pid": "1001", "pkey": pkey}}
	n, err := prov.VerifyNotification(context.Background(), body.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != payment.NotificationStatusSuccess || n.OrderID != "order-1" || n.Amount != 10 {
		t.Fatalf("notification = %+v", n)
	}
}

func TestEasyPayConfirmNotificationRequiresPaidQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":1,"trade_status":"WAITING","money":"10.00","trade_no":"upstream-1"}`)
	}))
	defer srv.Close()
	prov, err := NewEasyPay("inst", map[string]string{
		"pid": "1001", "pkey": "merchant-secret", "apiBase": srv.URL,
		"notifyUrl": "https://site.example/notify", "returnUrl": "https://site.example/payment/result",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = prov.ConfirmNotification(context.Background(), &payment.PaymentNotification{
		OrderID: "order-1", TradeNo: "upstream-1", Amount: 10, Status: payment.NotificationStatusSuccess,
	})
	if err == nil {
		t.Fatal("unpaid query must not confirm the notification")
	}
}

func TestEasyPayConfirmNotificationAcceptsMatchingPaidQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":1,"trade_status":"TRADE_SUCCESS","money":"10.00","trade_no":"upstream-1"}`)
	}))
	defer srv.Close()
	prov, err := NewEasyPay("inst", map[string]string{
		"pid": "1001", "pkey": "merchant-secret", "apiBase": srv.URL,
		"notifyUrl": "https://site.example/notify", "returnUrl": "https://site.example/payment/result",
	})
	if err != nil {
		t.Fatal(err)
	}
	n := &payment.PaymentNotification{
		OrderID: "order-1", TradeNo: "upstream-1", Amount: 10, Status: payment.NotificationStatusSuccess,
	}
	if err := prov.ConfirmNotification(context.Background(), n); err != nil {
		t.Fatal(err)
	}
}
