import type { ApiError } from './api'

// Stable backend error codes → i18n keys under errors.*. Codes missing here
// fall back to the backend's English message.
const ERROR_KEYS: Record<string, string> = {
  short_code_length_range: 'shortCodeLengthRange',
  invalid_base_url: 'invalidBaseUrl',
  invalid_extra_base_url: 'invalidExtraBaseUrl',
  invalid_reserved_code: 'invalidReservedCode',
  invalid_ip_block: 'invalidIpBlock',
  negative_login_rate: 'negativeLoginRate',
  negative_link_rate: 'negativeLinkRate',
  invalid_session_ttl: 'invalidSessionTtl',
  invalid_log_level: 'invalidLogLevel',
  token_taken: 'tokenTaken',
  pattern_taken: 'patternTaken',
}

/**
 * Translate an API error for a toast: known codes map to their errors.*
 * message with the backend's params interpolated; unknown codes show the
 * backend's original message.
 */
export function apiErrorMessage(err: ApiError, t: (key: string, opts?: Record<string, unknown>) => string): string {
  const key = ERROR_KEYS[err.code]
  if (key) return t(`errors.${key}`, err.params ?? {})
  return err.message
}
