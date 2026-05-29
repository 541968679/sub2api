# 支付系统

## 数据模型

| 实体/类型 | 位置 | 说明 |
|-----------|------|------|
| PaymentOrder | `backend/ent/schema/paymentorder.go` | 用户充值/订阅订单，记录订单状态、金额、支付方式、服务商实例快照、回调交易号 |
| PaymentProviderInstance | `backend/ent/schema/paymentproviderinstance.go` | 支付服务商实例，含 provider_key、supported_types、limits、payment_mode、配置 JSON |
| PaymentConfig | `backend/internal/service/payment_config_service.go` | 系统级支付配置：金额范围、订单超时、可见方式路由、汇率、首充赠送等 |
| CreatePaymentRequest/Response | `backend/internal/payment/types.go` | Provider 抽象层的下单请求/响应，支持 QR、PayURL、Stripe client secret、微信 OAuth/JSAPI |
| ResumeTokenClaims | `backend/internal/service/payment_resume_service.go` | 支付结果页/微信支付 OAuth 回跳恢复上下文的签名令牌 |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| 用户 Handler | `backend/internal/handler/payment_handler.go` | 下单、查单、取消、验单、退款申请 |
| Admin Handler | `backend/internal/handler/admin/payment_handler.go` | 服务商实例、订单管理、退款管理 |
| 服务层 | `backend/internal/service/payment_order.go` | 下单主流程、实例选择、订单落库、调用 provider、错误分类 |
| 配置服务 | `backend/internal/service/payment_config_*.go` | 支付配置、服务商实例 CRUD、限额与可见支付方式 |
| 可见方式路由 | `backend/internal/service/payment_resume_service.go`, `payment_visible_method_instances.go` | `支付宝`/`微信支付` 前台按钮路由到官方或 EasyPay 实例 |
| Provider | `backend/internal/payment/provider/*.go` | EasyPay、支付宝、微信官方、Stripe 对接 |
| 前端支付页 | `frontend/src/views/user/PaymentView.vue` | 充值/订阅下单、微信 OAuth/JSAPI、二维码/跳转/Stripe 启动 |
| 前端支付流工具 | `frontend/src/components/payment/paymentFlow.ts` | 规范化可见方式、构造下单 payload、决定支付启动方式 |
| 前端 Provider 配置 | `frontend/src/components/payment/providerConfig.ts` | 管理端服务商支持类型、配置字段、回调路径 |

## 核心流程

```
PaymentView
  -> POST /api/v1/payment/orders
  -> PaymentHandler.CreateOrder
  -> PaymentService.CreateOrder
     -> validateOrderInput / checkCancelRateLimit / checkPendingLimit / checkDailyLimit
     -> visibleMethodLoadBalancer.SelectInstance
     -> maybeBuildWeChatOAuthRequiredResponseForSelection
     -> createOrderInTx
     -> provider.CreateProvider(...).CreatePayment(...)
     -> update order with pay_url / qr_code / trade_no
     -> frontend decidePaymentLaunch(...)
```

前台只展示 `alipay`、`wxpay`、`stripe` 等可见方式。`alipay` 和 `wxpay` 会通过设置项 `payment_visible_method_*_source` 路由到官方 provider 或 EasyPay provider。不要让前端直接暴露 `official_*` 来源。

## 重要机制

- 微信官方支付按场景分流：有 OpenID 时走 JSAPI；移动端无 OpenID 时先走 H5；桌面端走 Native 二维码。
- 微信内浏览器下单如果缺少 OpenID，后端先返回 `oauth_required`，前端跳 `/api/v1/auth/oauth/wechat/payment/start`，回调携带签名 `wechat_resume_token` 后再次下单。
- 前端移动端兜底到扫码时会显式发送 `is_mobile=false`、`is_wechat_browser=false`、`force_native_qr=true`，并在 OAuth 回跳场景继续带 `wechat_resume_token`，让后端先恢复金额/套餐上下文再强制走 Native QR。
- 官方微信 provider 配置支持可选 `mpAppId`、`h5AppName`、`h5AppUrl`。`mpAppId` 必须和微信支付 OAuth 获取到的 OpenID 所属公众号一致，否则 JSAPI 会失败。
- 移动端微信 H5 未开通或无权限时，provider 会自动尝试 Native 二维码兜底；如果兜底也失败，服务层把常见 H5/JSAPI 上游错误映射成明确前端错误码。
- 支付回调必须使用外网 HTTPS 可访问地址。微信官方回调路径为 `/api/v1/payment/webhook/wxpay`。
- 支付恢复令牌签名优先用 `PAYMENT_RESUME_SIGNING_KEY`，兼容旧的 TOTP 加密密钥校验。

## 已知陷阱

- 管理端新增/编辑微信官方实例时，支付商户 `appId` 不一定等于公众号 AppID；JSAPI 场景需要填写 `mpAppId`。
- 移动端/微信内失败后不能只依赖 User-Agent 改走二维码；必须让创建订单请求显式覆盖 `is_wechat_browser`，否则后端会再次返回 OAuth/JSAPI。
- 如果生产反代没有传 `X-Forwarded-Proto` / `X-Forwarded-Host`，OAuth 回调 URL 可能生成成内网或 http 地址。优先在系统设置里配置正确的 `api_base_url`。
- `payment_mode` 目前主要影响 EasyPay 的二维码/弹窗模式；官方微信的展示模式由 provider 返回的 `qr_code` / `pay_url` / `jsapi` 决定。
- 服务商配置中的敏感字段在编辑时不回显，空提交表示保留原值。
- 有未完成订单时，关键支付身份字段受保护，不能直接修改。
