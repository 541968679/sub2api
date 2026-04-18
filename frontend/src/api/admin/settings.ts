/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 */

import { apiClient } from '../client'
import type { CustomMenuItem, CustomEndpoint } from '@/types'

export interface DefaultSubscriptionSetting {
  group_id: number
  validity_days: number
}

/**
 * System settings interface
 */
export interface SystemSettings {
  // Registration settings
  registration_enabled: boolean
  email_verify_enabled: boolean
  registration_email_suffix_whitelist: string[]
  promo_code_enabled: boolean
  password_reset_enabled: boolean
  frontend_url: string
  invitation_code_enabled: boolean
  totp_enabled: boolean // TOTP 双因素认证
  totp_encryption_key_configured: boolean // TOTP 加密密钥是否已配置
  // Default settings
  default_balance: number
  default_concurrency: number
  default_subscriptions: DefaultSubscriptionSetting[]
  // OEM settings
  site_name: string
  site_logo: string
  site_subtitle: string
  api_base_url: string
  contact_info: string
  doc_url: string
  home_content: string
  hide_ccs_import_button: boolean
  table_default_page_size: number
  table_page_size_options: number[]
  backend_mode_enabled: boolean
  custom_menu_items: CustomMenuItem[]
  custom_endpoints: CustomEndpoint[]
  // SMTP settings
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password_configured: boolean
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
  // Cloudflare Turnstile settings
  turnstile_enabled: boolean
  turnstile_site_key: string
  turnstile_secret_key_configured: boolean

  // LinuxDo Connect OAuth settings
  linuxdo_connect_enabled: boolean
  linuxdo_connect_client_id: string
  linuxdo_connect_client_secret_configured: boolean
  linuxdo_connect_redirect_url: string

  // Generic OIDC OAuth settings
  oidc_connect_enabled: boolean
  oidc_connect_provider_name: string
  oidc_connect_client_id: string
  oidc_connect_client_secret_configured: boolean
  oidc_connect_issuer_url: string
  oidc_connect_discovery_url: string
  oidc_connect_authorize_url: string
  oidc_connect_token_url: string
  oidc_connect_userinfo_url: string
  oidc_connect_jwks_url: string
  oidc_connect_scopes: string
  oidc_connect_redirect_url: string
  oidc_connect_frontend_redirect_url: string
  oidc_connect_token_auth_method: string
  oidc_connect_use_pkce: boolean
  oidc_connect_validate_id_token: boolean
  oidc_connect_allowed_signing_algs: string
  oidc_connect_clock_skew_seconds: number
  oidc_connect_require_email_verified: boolean
  oidc_connect_userinfo_email_path: string
  oidc_connect_userinfo_id_path: string
  oidc_connect_userinfo_username_path: string

  // Model fallback configuration
  enable_model_fallback: boolean
  fallback_model_anthropic: string
  fallback_model_openai: string
  fallback_model_gemini: string
  fallback_model_antigravity: string

  // Identity patch configuration (Claude -> Gemini)
  enable_identity_patch: boolean
  identity_patch_prompt: string

  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean
  ops_realtime_monitoring_enabled: boolean
  ops_query_mode_default: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds: number

  // Claude Code version check
  min_claude_code_version: string
  max_claude_code_version: string

  // 分组隔离
  allow_ungrouped_key_scheduling: boolean

  // Gateway forwarding behavior
  enable_fingerprint_unification: boolean
  enable_metadata_passthrough: boolean
  enable_cch_signing: boolean

  // Payment configuration
  payment_enabled: boolean
  payment_min_amount: number
  payment_max_amount: number
  payment_daily_limit: number
  payment_order_timeout_minutes: number
  payment_max_pending_orders: number
  payment_enabled_types: string[]
  payment_balance_disabled: boolean
  payment_load_balance_strategy: string
  payment_product_name_prefix: string
  payment_product_name_suffix: string
  payment_help_image_url: string
  payment_help_text: string
  payment_cny_per_usd: number
  payment_bonus_tiers: { min_amount: number; bonus_usd: number }[]
  payment_cancel_rate_limit_enabled: boolean
  payment_cancel_rate_limit_max: number
  payment_cancel_rate_limit_window: number
  payment_cancel_rate_limit_unit: string
  payment_cancel_rate_limit_window_mode: string
}

export interface UpdateSettingsRequest {
  registration_enabled?: boolean
  email_verify_enabled?: boolean
  registration_email_suffix_whitelist?: string[]
  promo_code_enabled?: boolean
  password_reset_enabled?: boolean
  frontend_url?: string
  invitation_code_enabled?: boolean
  totp_enabled?: boolean // TOTP 双因素认证
  default_balance?: number
  default_concurrency?: number
  default_subscriptions?: DefaultSubscriptionSetting[]
  site_name?: string
  site_logo?: string
  site_subtitle?: string
  api_base_url?: string
  contact_info?: string
  doc_url?: string
  home_content?: string
  hide_ccs_import_button?: boolean
  table_default_page_size?: number
  table_page_size_options?: number[]
  backend_mode_enabled?: boolean
  custom_menu_items?: CustomMenuItem[]
  custom_endpoints?: CustomEndpoint[]
  smtp_host?: string
  smtp_port?: number
  smtp_username?: string
  smtp_password?: string
  smtp_from_email?: string
  smtp_from_name?: string
  smtp_use_tls?: boolean
  turnstile_enabled?: boolean
  turnstile_site_key?: string
  turnstile_secret_key?: string
  linuxdo_connect_enabled?: boolean
  linuxdo_connect_client_id?: string
  linuxdo_connect_client_secret?: string
  linuxdo_connect_redirect_url?: string
  oidc_connect_enabled?: boolean
  oidc_connect_provider_name?: string
  oidc_connect_client_id?: string
  oidc_connect_client_secret?: string
  oidc_connect_issuer_url?: string
  oidc_connect_discovery_url?: string
  oidc_connect_authorize_url?: string
  oidc_connect_token_url?: string
  oidc_connect_userinfo_url?: string
  oidc_connect_jwks_url?: string
  oidc_connect_scopes?: string
  oidc_connect_redirect_url?: string
  oidc_connect_frontend_redirect_url?: string
  oidc_connect_token_auth_method?: string
  oidc_connect_use_pkce?: boolean
  oidc_connect_validate_id_token?: boolean
  oidc_connect_allowed_signing_algs?: string
  oidc_connect_clock_skew_seconds?: number
  oidc_connect_require_email_verified?: boolean
  oidc_connect_userinfo_email_path?: string
  oidc_connect_userinfo_id_path?: string
  oidc_connect_userinfo_username_path?: string
  enable_model_fallback?: boolean
  fallback_model_anthropic?: string
  fallback_model_openai?: string
  fallback_model_gemini?: string
  fallback_model_antigravity?: string
  enable_identity_patch?: boolean
  identity_patch_prompt?: string
  ops_monitoring_enabled?: boolean
  ops_realtime_monitoring_enabled?: boolean
  ops_query_mode_default?: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds?: number
  min_claude_code_version?: string
  max_claude_code_version?: string
  allow_ungrouped_key_scheduling?: boolean
  enable_fingerprint_unification?: boolean
  enable_metadata_passthrough?: boolean
  enable_cch_signing?: boolean
  // Payment configuration
  payment_enabled?: boolean
  payment_min_amount?: number
  payment_max_amount?: number
  payment_daily_limit?: number
  payment_order_timeout_minutes?: number
  payment_max_pending_orders?: number
  payment_enabled_types?: string[]
  payment_balance_disabled?: boolean
  payment_load_balance_strategy?: string
  payment_product_name_prefix?: string
  payment_product_name_suffix?: string
  payment_help_image_url?: string
  payment_help_text?: string
  payment_cny_per_usd?: number
  payment_bonus_tiers?: { min_amount: number; bonus_usd: number }[]
  payment_cancel_rate_limit_enabled?: boolean
  payment_cancel_rate_limit_max?: number
  payment_cancel_rate_limit_window?: number
  payment_cancel_rate_limit_unit?: string
  payment_cancel_rate_limit_window_mode?: string
}

/**
 * Get all system settings
 * @returns System settings
 */
export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>('/admin/settings')
  return data
}

/**
 * Update system settings
 * @param settings - Partial settings to update
 * @returns Updated settings
 */
export async function updateSettings(settings: UpdateSettingsRequest): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>('/admin/settings', settings)
  return data
}

// Backend bool/string fields in UpdateSettingsRequest are non-pointer — a missing
// key is unmarshalled as false/"" and then persisted, wiping whatever was stored.
// Pages that only touch a subset (e.g. RechargeConfigView) must merge the current
// settings into the request. Secret-like fields are omitted so the backend's
// "skip overwrite when empty" guard keeps the stored value.
export function systemSettingsToUpdateRequest(s: SystemSettings): UpdateSettingsRequest {
  return {
    registration_enabled: s.registration_enabled,
    email_verify_enabled: s.email_verify_enabled,
    registration_email_suffix_whitelist: s.registration_email_suffix_whitelist,
    promo_code_enabled: s.promo_code_enabled,
    password_reset_enabled: s.password_reset_enabled,
    frontend_url: s.frontend_url,
    invitation_code_enabled: s.invitation_code_enabled,
    totp_enabled: s.totp_enabled,
    default_balance: s.default_balance,
    default_concurrency: s.default_concurrency,
    default_subscriptions: s.default_subscriptions,
    site_name: s.site_name,
    site_logo: s.site_logo,
    site_subtitle: s.site_subtitle,
    api_base_url: s.api_base_url,
    contact_info: s.contact_info,
    doc_url: s.doc_url,
    home_content: s.home_content,
    hide_ccs_import_button: s.hide_ccs_import_button,
    table_default_page_size: s.table_default_page_size,
    table_page_size_options: s.table_page_size_options,
    backend_mode_enabled: s.backend_mode_enabled,
    custom_menu_items: s.custom_menu_items,
    custom_endpoints: s.custom_endpoints,
    smtp_host: s.smtp_host,
    smtp_port: s.smtp_port,
    smtp_username: s.smtp_username,
    smtp_from_email: s.smtp_from_email,
    smtp_from_name: s.smtp_from_name,
    smtp_use_tls: s.smtp_use_tls,
    turnstile_enabled: s.turnstile_enabled,
    turnstile_site_key: s.turnstile_site_key,
    linuxdo_connect_enabled: s.linuxdo_connect_enabled,
    linuxdo_connect_client_id: s.linuxdo_connect_client_id,
    linuxdo_connect_redirect_url: s.linuxdo_connect_redirect_url,
    oidc_connect_enabled: s.oidc_connect_enabled,
    oidc_connect_provider_name: s.oidc_connect_provider_name,
    oidc_connect_client_id: s.oidc_connect_client_id,
    oidc_connect_issuer_url: s.oidc_connect_issuer_url,
    oidc_connect_discovery_url: s.oidc_connect_discovery_url,
    oidc_connect_authorize_url: s.oidc_connect_authorize_url,
    oidc_connect_token_url: s.oidc_connect_token_url,
    oidc_connect_userinfo_url: s.oidc_connect_userinfo_url,
    oidc_connect_jwks_url: s.oidc_connect_jwks_url,
    oidc_connect_scopes: s.oidc_connect_scopes,
    oidc_connect_redirect_url: s.oidc_connect_redirect_url,
    oidc_connect_frontend_redirect_url: s.oidc_connect_frontend_redirect_url,
    oidc_connect_token_auth_method: s.oidc_connect_token_auth_method,
    oidc_connect_use_pkce: s.oidc_connect_use_pkce,
    oidc_connect_validate_id_token: s.oidc_connect_validate_id_token,
    oidc_connect_allowed_signing_algs: s.oidc_connect_allowed_signing_algs,
    oidc_connect_clock_skew_seconds: s.oidc_connect_clock_skew_seconds,
    oidc_connect_require_email_verified: s.oidc_connect_require_email_verified,
    oidc_connect_userinfo_email_path: s.oidc_connect_userinfo_email_path,
    oidc_connect_userinfo_id_path: s.oidc_connect_userinfo_id_path,
    oidc_connect_userinfo_username_path: s.oidc_connect_userinfo_username_path,
    enable_model_fallback: s.enable_model_fallback,
    fallback_model_anthropic: s.fallback_model_anthropic,
    fallback_model_openai: s.fallback_model_openai,
    fallback_model_gemini: s.fallback_model_gemini,
    fallback_model_antigravity: s.fallback_model_antigravity,
    enable_identity_patch: s.enable_identity_patch,
    identity_patch_prompt: s.identity_patch_prompt,
    ops_monitoring_enabled: s.ops_monitoring_enabled,
    ops_realtime_monitoring_enabled: s.ops_realtime_monitoring_enabled,
    ops_query_mode_default: s.ops_query_mode_default,
    ops_metrics_interval_seconds: s.ops_metrics_interval_seconds,
    min_claude_code_version: s.min_claude_code_version,
    max_claude_code_version: s.max_claude_code_version,
    allow_ungrouped_key_scheduling: s.allow_ungrouped_key_scheduling,
    enable_fingerprint_unification: s.enable_fingerprint_unification,
    enable_metadata_passthrough: s.enable_metadata_passthrough,
    enable_cch_signing: s.enable_cch_signing,
    payment_enabled: s.payment_enabled,
    payment_min_amount: s.payment_min_amount,
    payment_max_amount: s.payment_max_amount,
    payment_daily_limit: s.payment_daily_limit,
    payment_order_timeout_minutes: s.payment_order_timeout_minutes,
    payment_max_pending_orders: s.payment_max_pending_orders,
    payment_enabled_types: s.payment_enabled_types,
    payment_balance_disabled: s.payment_balance_disabled,
    payment_load_balance_strategy: s.payment_load_balance_strategy,
    payment_product_name_prefix: s.payment_product_name_prefix,
    payment_product_name_suffix: s.payment_product_name_suffix,
    payment_help_image_url: s.payment_help_image_url,
    payment_help_text: s.payment_help_text,
    payment_cny_per_usd: s.payment_cny_per_usd,
    payment_bonus_tiers: s.payment_bonus_tiers,
    payment_cancel_rate_limit_enabled: s.payment_cancel_rate_limit_enabled,
    payment_cancel_rate_limit_max: s.payment_cancel_rate_limit_max,
    payment_cancel_rate_limit_window: s.payment_cancel_rate_limit_window,
    payment_cancel_rate_limit_unit: s.payment_cancel_rate_limit_unit,
    payment_cancel_rate_limit_window_mode: s.payment_cancel_rate_limit_window_mode,
  }
}

/**
 * Test SMTP connection request
 */
export interface TestSmtpRequest {
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_use_tls: boolean
}

/**
 * Test SMTP connection with provided config
 * @param config - SMTP configuration to test
 * @returns Test result message
 */
export async function testSmtpConnection(config: TestSmtpRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/settings/test-smtp', config)
  return data
}

/**
 * Send test email request
 */
export interface SendTestEmailRequest {
  email: string
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
}

/**
 * Send test email with provided SMTP config
 * @param request - Email address and SMTP config
 * @returns Test result message
 */
export async function sendTestEmail(request: SendTestEmailRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    '/admin/settings/send-test-email',
    request
  )
  return data
}

/**
 * Admin API Key status response
 */
export interface AdminApiKeyStatus {
  exists: boolean
  masked_key: string
}

/**
 * Get admin API key status
 * @returns Status indicating if key exists and masked version
 */
export async function getAdminApiKey(): Promise<AdminApiKeyStatus> {
  const { data } = await apiClient.get<AdminApiKeyStatus>('/admin/settings/admin-api-key')
  return data
}

/**
 * Regenerate admin API key
 * @returns The new full API key (only shown once)
 */
export async function regenerateAdminApiKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>('/admin/settings/admin-api-key/regenerate')
  return data
}

/**
 * Delete admin API key
 * @returns Success message
 */
export async function deleteAdminApiKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/admin/settings/admin-api-key')
  return data
}

// ==================== Overload Cooldown Settings ====================

/**
 * Overload cooldown settings interface (529 handling)
 */
export interface OverloadCooldownSettings {
  enabled: boolean
  cooldown_minutes: number
}

export async function getOverloadCooldownSettings(): Promise<OverloadCooldownSettings> {
  const { data } = await apiClient.get<OverloadCooldownSettings>('/admin/settings/overload-cooldown')
  return data
}

export async function updateOverloadCooldownSettings(
  settings: OverloadCooldownSettings
): Promise<OverloadCooldownSettings> {
  const { data } = await apiClient.put<OverloadCooldownSettings>(
    '/admin/settings/overload-cooldown',
    settings
  )
  return data
}

// ==================== Stream Timeout Settings ====================

/**
 * Stream timeout settings interface
 */
export interface StreamTimeoutSettings {
  enabled: boolean
  action: 'temp_unsched' | 'error' | 'none'
  temp_unsched_minutes: number
  threshold_count: number
  threshold_window_minutes: number
}

/**
 * Get stream timeout settings
 * @returns Stream timeout settings
 */
export async function getStreamTimeoutSettings(): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.get<StreamTimeoutSettings>('/admin/settings/stream-timeout')
  return data
}

/**
 * Update stream timeout settings
 * @param settings - Stream timeout settings to update
 * @returns Updated settings
 */
export async function updateStreamTimeoutSettings(
  settings: StreamTimeoutSettings
): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.put<StreamTimeoutSettings>(
    '/admin/settings/stream-timeout',
    settings
  )
  return data
}

// ==================== Rectifier Settings ====================

/**
 * Rectifier settings interface
 */
export interface RectifierSettings {
  enabled: boolean
  thinking_signature_enabled: boolean
  thinking_budget_enabled: boolean
  apikey_signature_enabled: boolean
  apikey_signature_patterns: string[]
}

/**
 * Get rectifier settings
 * @returns Rectifier settings
 */
export async function getRectifierSettings(): Promise<RectifierSettings> {
  const { data } = await apiClient.get<RectifierSettings>('/admin/settings/rectifier')
  return data
}

/**
 * Update rectifier settings
 * @param settings - Rectifier settings to update
 * @returns Updated settings
 */
export async function updateRectifierSettings(
  settings: RectifierSettings
): Promise<RectifierSettings> {
  const { data } = await apiClient.put<RectifierSettings>(
    '/admin/settings/rectifier',
    settings
  )
  return data
}

// ==================== Beta Policy Settings ====================

/**
 * Beta policy rule interface
 */
export interface BetaPolicyRule {
  beta_token: string
  action: 'pass' | 'filter' | 'block'
  scope: 'all' | 'oauth' | 'apikey' | 'bedrock'
  error_message?: string
  model_whitelist?: string[]
  fallback_action?: 'pass' | 'filter' | 'block'
  fallback_error_message?: string
}

/**
 * Beta policy settings interface
 */
export interface BetaPolicySettings {
  rules: BetaPolicyRule[]
}

/**
 * Get beta policy settings
 * @returns Beta policy settings
 */
export async function getBetaPolicySettings(): Promise<BetaPolicySettings> {
  const { data } = await apiClient.get<BetaPolicySettings>('/admin/settings/beta-policy')
  return data
}

/**
 * Update beta policy settings
 * @param settings - Beta policy settings to update
 * @returns Updated settings
 */
export async function updateBetaPolicySettings(
  settings: BetaPolicySettings
): Promise<BetaPolicySettings> {
  const { data } = await apiClient.put<BetaPolicySettings>(
    '/admin/settings/beta-policy',
    settings
  )
  return data
}

export const settingsAPI = {
  getSettings,
  updateSettings,
  systemSettingsToUpdateRequest,
  testSmtpConnection,
  sendTestEmail,
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey,
  getOverloadCooldownSettings,
  updateOverloadCooldownSettings,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings
}

export default settingsAPI
