import { execFileSync } from 'node:child_process'
import { describe, expect, it } from 'vitest'
import { buildGuideRequest, normalizeGuideBaseUrl, type GuideProtocol } from '../docsGuideExamples'

describe('documentation request examples', () => {
  it('uses exactly one API suffix while preserving custom path prefixes', () => {
    expect(normalizeGuideBaseUrl(' https://example.test/proxy/v1/// ')).toBe('https://example.test/proxy')
    expect(normalizeGuideBaseUrl('http://example.test:8080/')).toBe('http://example.test:8080')
  })

  for (const protocol of ['messages', 'responses', 'chat'] as GuideProtocol[]) {
    it(`executes the ${protocol} Unix example with an intact JSON body and literal model`, () => {
      // Capture curl arguments without network access. This checks actual shell
      // parsing, including quotes, rather than comparing generated strings.
      const model = `model'$(printf UNEXPECTED)'\\"`
      const script = buildGuideRequest('https://example.test/proxy/v1/', model, protocol, 'unix')
      const args = execFileSync('bash', ['-c', 'curl() { printf "%s\\0" "$@"; }\n' + script], { encoding: 'utf8' }).split('\0').filter(Boolean)
      const path = { messages: 'messages', responses: 'responses', chat: 'chat/completions' }[protocol]
      expect(args).toContain(`https://example.test/proxy/v1/${path}`)
      expect(args).toContain('Authorization: Bearer sk-your-key')
      const body = JSON.parse(args[args.indexOf('-d') + 1])
      expect(body.model).toBe(model)
      if (protocol === 'responses') {
        expect(body.input).toBe('Reply with OK.')
        expect(body.messages).toBeUndefined()
      } else {
        expect(body.messages).toEqual([{ role: 'user', content: 'Reply with OK.' }])
      }
      expect(args.includes('anthropic-version: 2023-06-01')).toBe(protocol === 'messages')
      if (protocol === 'messages') expect(body.max_tokens).toBe(64)
    })

    it(`provides ${protocol} PowerShell here-string JSON and UTF-8 bytes without Unix continuations`, () => {
      const script = buildGuideRequest('http://example.test:8080', 'custom-model', protocol, 'windows')
      const body = JSON.parse(script.split("$body = @'\n")[1].split("\n'@")[0])
      expect(body.model).toBe('custom-model')
      expect(script).toContain('Invoke-RestMethod -Method Post')
      expect(script).toContain('[System.Text.Encoding]::UTF8.GetBytes($body)')
      expect(script).not.toContain('\\\n')
    })
  }
})
