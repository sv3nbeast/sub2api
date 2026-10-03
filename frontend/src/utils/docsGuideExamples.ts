export type GuideOS = 'unix' | 'windows'
export type GuideProtocol = 'messages' | 'responses' | 'chat'

// Public settings sometimes contain the OpenAI suffix. All guide URLs start
// from the gateway root, including installations under a path prefix.
export function normalizeGuideBaseUrl(url: string): string {
  return url.trim().replace(/\/+$/, '').replace(/\/v1$/i, '')
}

function shellLiteral(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`
}

function powershellLiteral(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

export function buildGuideRequest(baseUrl: string, model: string, protocol: GuideProtocol, os: GuideOS): string {
  const path = { messages: '/v1/messages', responses: '/v1/responses', chat: '/v1/chat/completions' }[protocol]
  const body = JSON.stringify(protocol === 'responses'
    ? { model, input: 'Reply with OK.' }
    : { model, ...(protocol === 'messages' ? { max_tokens: 64 } : {}), messages: [{ role: 'user', content: 'Reply with OK.' }] }, null, 2)
  const url = `${normalizeGuideBaseUrl(baseUrl)}${path}`
  if (os === 'windows') {
    return `$apiKey = 'sk-your-key'\n$body = @'\n${body}\n'@\nInvoke-RestMethod -Method Post -Uri ${powershellLiteral(url)} \`\n  -Headers @{ Authorization = "Bearer $apiKey"${protocol === 'messages' ? "; 'anthropic-version' = '2023-06-01'" : ''} } \`\n  -ContentType 'application/json' \`\n  -Body ([System.Text.Encoding]::UTF8.GetBytes($body))`
  }
  const lines = [
    `curl -sS ${shellLiteral(url)}`,
    '  -H "Authorization: Bearer $API_KEY"',
    "  -H 'Content-Type: application/json'",
    ...(protocol === 'messages' ? ["  -H 'anthropic-version: 2023-06-01'"] : []),
    `  -d ${shellLiteral(body)}`,
  ]
  return "API_KEY='sk-your-key'\n" + lines.join(` ${String.fromCharCode(92)}\n`)
}
