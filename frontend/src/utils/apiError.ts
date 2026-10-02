/**
 * Centralized API error message extraction
 *
 * The API client interceptor rejects with a plain object: { status, code, message, error }
 * This utility extracts the user-facing message from any error shape.
 */

interface ApiErrorLike {
  status?: number
  code?: number | string
  message?: string
  error?: string
  reason?: string
  metadata?: Record<string, unknown>
  response?: {
    data?: {
      detail?: string
      message?: string
      code?: number | string
    }
  }
}

/**
 * Extract the error code from an API error object.
 *
 * Prefers the semantic identifiers (`reason`, or a string `code`) over the
 * numeric HTTP status the interceptor copies into `code`. Domain failures are
 * enveloped as `{ code: 400, message, reason: "INVALID_AMOUNT" }`, so reading
 * `code` first yields "400" and every i18n lookup keyed on the reason misses.
 * Numeric codes are still returned as a last resort so callers that switch on
 * an HTTP status keep working.
 */
export function extractApiErrorCode(err: unknown): string | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  const candidates = [e.reason, e.code, e.response?.data?.code]
  for (const candidate of candidates) {
    // Skip numeric HTTP statuses while a semantic identifier may still follow.
    if (typeof candidate === 'number') continue
    if (typeof candidate === 'string' && candidate.trim() !== '') return candidate
  }
  const numeric = candidates.find(
    (candidate) => typeof candidate === 'number' || (typeof candidate === 'string' && candidate.trim() !== ''),
  )
  return numeric != null ? String(numeric) : undefined
}

/**
 * Extract a displayable error message from an API error.
 *
 * @param err - The caught error (unknown type)
 * @param fallback - Fallback message if none can be extracted (use t('common.error') or similar)
 * @param i18nMap - Optional map of error codes to i18n translated strings
 */
export function extractApiErrorMessage(
  err: unknown,
  fallback = 'Unknown error',
  i18nMap?: Record<string, string>,
): string {
  if (!err) return fallback

  // Try i18n mapping by error code first
  if (i18nMap) {
    const code = extractApiErrorCode(err)
    if (code && i18nMap[code]) return i18nMap[code]
  }

  // Plain object from API client interceptor (most common case)
  if (typeof err === 'object' && err !== null) {
    const e = err as ApiErrorLike
    // Interceptor shape: { message, error }
    if (e.message) return e.message
    if (e.error) return e.error
    // Legacy axios shape: { response.data.detail }
    if (e.response?.data?.detail) return e.response.data.detail
    if (e.response?.data?.message) return e.response.data.message
  }

  // Standard Error
  if (err instanceof Error) return err.message

  // Last resort
  const str = String(err)
  return str === '[object Object]' ? fallback : str
}

export function extractI18nErrorMessage(
  err: unknown,
  fallbackOrTranslator: string | ((key: string, params?: Record<string, unknown>) => string) = 'Unknown error',
  namespaceOrI18nMap?: string | Record<string, string>,
  fallbackMaybe?: string,
): string {
  if (typeof fallbackOrTranslator === 'function') {
    const t = fallbackOrTranslator
    const namespace = typeof namespaceOrI18nMap === 'string' ? namespaceOrI18nMap : ''
    const fallback = fallbackMaybe ?? 'Unknown error'
    const code = extractApiErrorCode(err)
    if (code && namespace) {
      const metadata = typeof err === 'object' && err !== null
        ? (err as ApiErrorLike).metadata
        : undefined
      const translated = t(`${namespace}.${code}`, metadata)
      if (translated && translated !== `${namespace}.${code}`) {
        return translated
      }
    }
    return extractApiErrorMessage(err, fallback)
  }

  return extractApiErrorMessage(
    err,
    fallbackOrTranslator,
    typeof namespaceOrI18nMap === 'object' ? namespaceOrI18nMap : undefined,
  )
}
