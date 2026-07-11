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

## 混合/打包订阅 (Bundle Subscription)

一个订阅套餐可以打包多个订阅型分组，一次购买扇出成 **N 条独立 `user_subscription`**（每个成员分组一条），各组用各组自身的 `daily/weekly/monthly_limit_usd` 各自计额度。用户通过切换 API key 的分组（或每组各用一把 key）访问。**网关/计费/额度/缓存热路径完全不变**（仍按 `(user_id, group_id)`），成员组仍单平台。

数据与流程：

- `subscription_plans.member_group_ids` / `payment_orders.member_group_ids`（JSONB，默认 `[]`，迁移 `168`）。空 = 旧单组套餐/订单，行为不变。
- 有效成员集 = `unique(group_id ∪ member_group_ids)`，`group_id` 为主/代表组。helper `service.PlanMemberGroupIDs(plan)`（`payment_config_plans.go`）。
- 下单：`createOrderInTx`（`payment_order.go`）把成员集快照到订单（冻结，避免下单后改套餐影响履约）。
- 履约：`doSub`（`payment_fulfillment.go`）逐组扇出，逐组幂等审计 `SUBSCRIPTION_SUCCESS:<gid>`，死的非主成员写 `SUBSCRIPTION_MEMBER_SKIPPED:<gid>` 跳过；全部成员成功后才写无后缀 `SUBSCRIPTION_SUCCESS` 触发 `markCompleted`。复用现有 `SubscriptionService.AssignOrExtendSubscription`（按 `(user,group)` 幂等）。
- 每个成员组的订阅赋权与 `SUBSCRIPTION_SUCCESS:<gid>` 审计在同一外层事务内提交；订阅 L1/Redis 缓存只在提交成功后同步失效。若失效失败后重试，已审计分组仍会再次执行缓存失效，不会重复续期。
- 校验：管理端 `CreatePlan/UpdatePlan` 的 `member_group_ids` 经 `normalizeMemberGroupIDs` 规范化（丢 ≤0、去重、移除主组、必须是存在的订阅型分组、上限 10）。
- 对外展示：`GetPlans`/`GetCheckoutInfo`（`handler/payment_handler.go`）暴露 `member_group_ids` + `member_groups`（每成员的 platform/name/limits/scopes）。前端 `SubscriptionPlanCard.vue` 当 `member_groups.length>1` 渲染"包含"区块，管理端 `PlanEditDialog.vue` 多选附加组。
- **退款未适配**：`payment_refund.go` 仅回滚主组（本部署未启用退款）。若启用，需对混合单做按组逐个回滚或禁止自助退款。
- 二级渠道（兑换码/分销/管理端按套餐直接分配）当前**未做 bundle 扇出**，仅主购买链路支持。

## 已知陷阱

- 管理端新增/编辑微信官方实例时，支付商户 `appId` 不一定等于公众号 AppID；JSAPI 场景需要填写 `mpAppId`。
- 移动端/微信内失败后不能只依赖 User-Agent 改走二维码；必须让创建订单请求显式覆盖 `is_wechat_browser`，否则后端会再次返回 OAuth/JSAPI。
- 如果生产反代没有传 `X-Forwarded-Proto` / `X-Forwarded-Host`，OAuth 回调 URL 可能生成成内网或 http 地址。优先在系统设置里配置正确的 `api_base_url`。
- `payment_mode` 目前主要影响 EasyPay 的二维码/弹窗模式；官方微信的展示模式由 provider 返回的 `qr_code` / `pay_url` / `jsapi` 决定。
- 服务商配置中的敏感字段在编辑时不回显，空提交表示保留原值。
- 有未完成订单时，关键支付身份字段受保护，不能直接修改。
# 2026-07 upstream alignment notes

- Airwallex is a first-class provider with an embedded checkout route. Provider currencies are validated and formatted with currency-specific decimal precision.
- Checkout confirmation amounts use the selected provider currency; do not reintroduce hard-coded CNY symbols in shared recharge/subscription totals.
- Refunds may remain `REFUND_PENDING` until Stripe, WeChat Pay, or Airwallex confirms the final state. Local pre-deduction is rolled back while pending and applied exactly once after confirmed success.
- Fulfillment uses a five-minute ownership lease on the order `updated_at` token. Fresh owners cannot be stolen; stale `recharging` work can be recovered, and an old owner cannot finalize after takeover.
- EasyPay supports administrator-defined payment methods in addition to exact built-in method names. Provider response sanitization is limited to fields that may contain transport NUL bytes and never mutates secrets.
- `subscription_usd_to_cny_rate` is an explicit opt-in. The default `0` preserves this fork's existing plan-price-as-charge behavior. When enabled, only CNY gateway charges use `plan price * rate`; stored plan price, bundle membership, subscription quota, distribution subscription-code cost, and balance recharge multiplier are unchanged.
- Bundle invariants remain protected: `member_group_ids` is snapshotted onto the order, fulfillment writes one `SUBSCRIPTION_SUCCESS:<gid>` audit per member, and only the aggregate success completes the order.
- Subscription usage-window maintenance is synchronous on API-key auth when a
  reset is due. Automatic resets compare the previously observed window start
  before clearing usage, then reload and revalidate the committed subscription;
  a stale CAS loser cannot authorize from a locally zeroed snapshot.
