import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DocsGuideView from '@/views/public/DocsGuideView.vue'

const { authStore, appStore, copyToClipboard } = vi.hoisted(() => ({
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    checkAuth: vi.fn(),
  },
  appStore: {
    siteName: 'SubAPIs',
    siteLogo: '',
    apiBaseUrl: 'https://fallback.example.com',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
    cachedPublicSettings: {
      site_name: 'SubAPIs',
      site_logo: '',
      api_base_url: 'https://gateway.example.com/',
      api_key_usage_config: {
        codex_model: 'gpt-current',
        codex_review_model: 'gpt-review',
        codex_reasoning_effort: 'high',
        codex_disable_response_storage: true,
        codex_network_access: 'enabled',
        codex_goals_enabled: true,
        codex_websocket_enabled: true,
        claude_code_attribution_header: 0,
        claude_code_default_model: 'claude-current',
      },
    },
  },
  copyToClipboard: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authStore,
  useAppStore: () => appStore,
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      tm: () => ['first instruction', 'second instruction'],
    }),
  }
})

const RouterLinkStub = {
  props: ['to'],
  template: '<a :data-to="typeof to === \'string\' ? to : to.path"><slot /></a>',
}

function mountView() {
  return mount(DocsGuideView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        PublicLayout: { template: '<div><slot /></div>' },
        LocaleSwitcher: true,
        Icon: true,
      },
    },
  })
}

describe('DocsGuideView', () => {
  beforeEach(() => {
    authStore.checkAuth.mockReset()
    appStore.fetchPublicSettings.mockReset()
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)
  })

  it('uses current public settings for endpoint and client configuration examples', async () => {
    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('https://gateway.example.com/v1/messages')
    expect(text).toContain('https://gateway.example.com/v1/responses')
    expect(text).toContain('ANTHROPIC_AUTH_TOKEN')
    expect(text).not.toContain('ANTHROPIC_API_KEY')
    expect(text).toContain('ANTHROPIC_BASE_URL')
    expect(text).toContain('"https://gateway.example.com"')
    expect(text).toContain('model = "gpt-current"')
    expect(text).toContain('review_model = "gpt-review"')
    expect(text).toContain('wire_api = "responses"')
    expect(text).toContain('supports_websockets = false')
    expect(text).toContain('cli_auth_credentials_store = "file"')
    expect(wrapper.get('[data-snippet="codex-config"] code').text()).toContain('base_url = "https://gateway.example.com"')
    expect(wrapper.get('[data-snippet="codex-config"] code').text()).not.toContain('base_url = "https://gateway.example.com/v1"')
    expect(text).not.toContain('windows_wsl_setup_acknowledged')
    expect(text).not.toContain('/v1beta')
    expect(text).not.toContain('/antigravity')
  })

  it('switches configuration file paths between Unix and Windows', async () => {
    const wrapper = mountView()

    expect(wrapper.text()).toContain('~/.claude/settings.json')
    expect(wrapper.text()).toContain('~/.codex/config.toml')

    await wrapper.get('input[value="windows"]').setValue(true)

    expect(wrapper.text()).toContain('%USERPROFILE%\\.claude\\settings.json')
    expect(wrapper.text()).toContain('%USERPROFILE%\\.codex\\config.toml')
    expect(wrapper.text()).toContain('curl.exe')
    expect(wrapper.get('[data-snippet="curl"]').text()).toContain('Invoke-RestMethod')
    expect(wrapper.get('[data-snippet="env-check"]').text()).not.toContain('-I')
  })

  it('keeps the chosen protocol, model, and request body consistent across platform changes', async () => {
    const wrapper = mountView()
    expect(wrapper.get('[data-snippet="curl"] code').text()).toContain('claude-current')
    expect(wrapper.get('[data-snippet="curl"] code').text()).toContain('anthropic-version')
    await wrapper.get('#docs-protocol').setValue('responses')
    await wrapper.get('#docs-model').setValue('custom-model')
    expect(wrapper.get('[data-snippet="curl"] code').text()).toContain('"input"')
    await wrapper.get('input[value="windows"]').setValue(true)
    const request = wrapper.get('[data-snippet="curl"] code').text()
    expect(request).toContain('custom-model')
    expect(request).not.toContain('anthropic-version')
    expect(request).toContain('https://gateway.example.com/v1/responses')
    await wrapper.get('#docs-protocol').setValue('messages')
    expect(wrapper.get('#docs-model').element).toHaveProperty('value', 'claude-current')
    await wrapper.get('#docs-protocol').setValue('responses')
    expect(wrapper.get('#docs-model').element).toHaveProperty('value', 'custom-model')
  })

  it('copies the visible snippet and does not announce success when copying fails', async () => {
    const wrapper = mountView()
    const code = wrapper.get('[data-snippet="curl"] code').text()
    copyToClipboard.mockResolvedValueOnce(false)
    await wrapper.get('[data-snippet="curl"] button').trigger('click')
    await flushPromises()
    expect(copyToClipboard).toHaveBeenCalledWith(code)
    expect(wrapper.get('[data-snippet="curl"] button').text()).toBe('common.copy')
    await wrapper.get('[data-snippet="curl"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-snippet="curl"] button').text()).toBe('common.copied')
    wrapper.unmount()
  })

  it('provides valid mobile and desktop anchors, an accessible code scroller, and distinct VS Code settings', () => {
    const wrapper = mountView()
    for (const link of wrapper.findAll('.docs-sidebar a, .docs-mobile-nav a')) {
      expect(wrapper.find(link.attributes('href')).exists()).toBe(true)
    }
    expect(wrapper.get('[data-snippet="vscode-settings"] code').text()).toContain('claudeCode.environmentVariables')
    expect(wrapper.get('[data-snippet="curl"] pre').attributes('tabindex')).toBe('0')
    expect(wrapper.get('[data-snippet="curl"] button').attributes('aria-label')).toBeTruthy()
    expect(wrapper.find('.docs-mobile-nav').exists()).toBe(true)
    expect(wrapper.find('[role="tablist"]').exists()).toBe(false)
  })
})
