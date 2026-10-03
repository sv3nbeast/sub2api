<template>
  <PublicLayout :show-chrome="false" content-class="public-ui-v2__content--flush">
    <div class="docs-page min-h-screen bg-white text-gray-950 dark:bg-dark-900 dark:text-white">
      <header class="docs-topbar">
        <div class="docs-topbar-inner">
          <RouterLink to="/home" class="docs-brand" :aria-label="siteName">
            <img v-if="siteLogo" :src="siteLogo" :alt="siteName" class="h-7 w-7 object-contain" />
            <span v-else class="docs-brand-mark">S</span>
            <span>{{ siteName }}</span>
            <span class="docs-brand-divider"></span>
            <span class="text-sm font-medium text-gray-500 dark:text-dark-300">{{ t('docsGuide.sidebar.title') }}</span>
          </RouterLink>

          <nav class="flex items-center gap-1">
            <RouterLink to="/home" class="docs-icon-link" :title="t('docsGuide.nav.home')">
              <Icon name="home" size="sm" />
              <span class="hidden sm:inline">{{ t('docsGuide.nav.home') }}</span>
            </RouterLink>
            <RouterLink to="/status" class="docs-icon-link" :title="t('docsGuide.nav.status')">
              <Icon name="chart" size="sm" />
              <span class="hidden sm:inline">{{ t('docsGuide.nav.status') }}</span>
            </RouterLink>
            <LocaleSwitcher />
            <button type="button" class="docs-icon-button" :title="themeLabel" :aria-label="themeLabel" @click="toggleTheme">
              <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
            </button>
          </nav>
        </div>
      </header>

      <div class="docs-main">
        <section class="docs-intro">
          <div class="min-w-0">
            <p class="docs-kicker">{{ t('docsGuide.hero.eyebrow') }}</p>
            <h1>{{ t('docsGuide.hero.title') }}</h1>
            <p class="docs-lead">{{ t('docsGuide.hero.description') }}</p>
            <div class="mt-5 flex flex-wrap gap-2">
              <RouterLink :to="isAuthenticated ? dashboardPath : '/register'" class="btn btn-primary px-4 py-2.5 text-sm">
                {{ isAuthenticated ? t('home.goToDashboard') : t('docsGuide.hero.primaryCta') }}
                <Icon name="arrowRight" size="sm" />
              </RouterLink>
              <RouterLink to="/model-plaza" class="btn btn-secondary px-4 py-2.5 text-sm">
                {{ t('docsGuide.sections.models.articles.market.title') }}
              </RouterLink>
            </div>
          </div>

          <div class="docs-base-url">
            <span>{{ t('docsGuide.hero.baseUrlLabel') }}</span>
            <code>{{ apiBaseUrl }}</code>
            <button type="button" :title="t('common.copy')" :aria-label="t('docsGuide.controls.copyBaseUrl')" @click="copySnippet('base-url', apiBaseUrl)">
              <Icon :name="copiedId === 'base-url' ? 'check' : 'copy'" size="sm" />
            </button>
          </div>
        </section>

        <details class="docs-mobile-nav">
          <summary>{{ t('docsGuide.sidebar.title') }}<Icon name="chevronDown" size="sm" /></summary>
          <nav :aria-label="t('docsGuide.sidebar.title')">
            <a v-for="link in docsNavigation.flatMap(group => group.links)" :key="link.id" :href="`#${link.id}`" @click="closeMobileNav">{{ link.title }}</a>
          </nav>
        </details>

        <fieldset class="docs-os-picker">
          <legend>{{ t('docsGuide.controls.operatingSystem') }}</legend>
          <div class="docs-segmented">
            <label :class="{ active: selectedOS === 'unix' }"><input v-model="selectedOS" type="radio" name="docs-os" value="unix" /><span>macOS / Linux</span></label>
            <label :class="{ active: selectedOS === 'windows' }"><input v-model="selectedOS" type="radio" name="docs-os" value="windows" /><span>Windows PowerShell</span></label>
          </div>
          <p>{{ t('docsGuide.controls.osHint') }}</p>
        </fieldset>

        <div class="docs-layout">
          <aside class="docs-sidebar">
            <nav :aria-label="t('docsGuide.sidebar.title')">
              <div v-for="group in docsNavigation" :key="group.title" class="docs-nav-group">
                <p>{{ group.title }}</p>
                <a v-for="link in group.links" :key="link.id" :href="`#${link.id}`">
                  <span>{{ link.badge }}</span>
                  {{ link.title }}
                </a>
              </div>
            </nav>
          </aside>

          <article class="docs-content">
            <section id="quick-start" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.quickStart.eyebrow')" :title="t('docsGuide.quickStart.title')" :description="t('docsGuide.quickStart.description')" />
              <ol class="docs-steps">
                <li v-for="step in quickStartSteps" :id="step.id" :key="step.id" class="scroll-mt-24">
                  <span class="docs-step-number">{{ step.step }}</span>
                  <div>
                    <h3>{{ step.title }}</h3>
                    <p>{{ step.description }}</p>
                    <ul>
                      <li v-for="item in step.items" :key="item"><Icon name="check" size="xs" /><span>{{ item }}</span></li>
                    </ul>
                  </div>
                </li>
              </ol>
            </section>

            <section id="keys-and-models" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.sections.models.eyebrow')" :title="t('docsGuide.sections.models.title')" :description="t('docsGuide.sections.models.description')" />
              <div class="docs-prose-grid">
                <DocTopic id="model-center" badge="01" icon="grid" :title="t('docsGuide.sections.models.articles.market.title')" :description="t('docsGuide.sections.models.articles.market.description')" :items="translationList('docsGuide.sections.models.articles.market.items')" :note="t('docsGuide.sections.models.articles.market.note')">
                  <RouterLink to="/model-plaza" class="docs-inline-link">{{ t('modelMarket.viewModelsAndPricing') }}<Icon name="arrowRight" size="xs" /></RouterLink>
                </DocTopic>
                <DocTopic id="token-groups" badge="02" icon="key" :title="t('docsGuide.sections.models.articles.groups.title')" :description="t('docsGuide.sections.models.articles.groups.description')" :items="translationList('docsGuide.sections.models.articles.groups.items')" :note="t('docsGuide.sections.models.articles.groups.note')" />
                <DocTopic id="pricing-and-quota" badge="03" icon="calculator" :title="t('docsGuide.sections.models.articles.pricing.title')" :description="t('docsGuide.sections.models.articles.pricing.description')" :items="translationList('docsGuide.sections.models.articles.pricing.items')" :note="t('docsGuide.sections.models.articles.pricing.note')" />
              </div>
            </section>

            <section id="endpoint-map" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.examples.eyebrow')" :title="t('docsGuide.examples.title')" :description="t('docsGuide.examples.description')" />
              <p class="docs-callout">{{ t('docsGuide.examples.addressHint', { baseUrl: apiBaseUrl }) }}</p>
              <div class="docs-endpoint-table">
                <div v-for="endpoint in endpoints" :key="endpoint.path"><span>{{ endpoint.name }}</span><code>{{ endpoint.path }}</code></div>
              </div>
              <div class="docs-example-controls">
                <label for="docs-example-os">{{ t('docsGuide.controls.operatingSystem') }}</label>
                <select id="docs-example-os" v-model="selectedOS"><option value="unix">macOS / Linux</option><option value="windows">Windows PowerShell</option></select>
                <label for="docs-protocol">{{ t('docsGuide.controls.protocol') }}</label>
                <select id="docs-protocol" v-model="selectedProtocol">
                  <option value="messages">Anthropic Messages</option>
                  <option value="responses">OpenAI Responses</option>
                  <option value="chat">OpenAI Chat Completions</option>
                </select>
                <label for="docs-model">{{ t('docsGuide.controls.model') }}</label>
                <input id="docs-model" v-model="requestModel" type="text" spellcheck="false" autocomplete="off" />
              </div>
              <p class="docs-callout">{{ t('docsGuide.examples.note') }}</p>
              <CodeBlock id="curl" :label="terminalLabel" :code="curlExample" :copied="copiedId === 'curl'" @copy="copySnippet('curl', curlExample)" />
            </section>

            <section id="client-setup" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.sections.cli.eyebrow')" :title="t('docsGuide.sections.cli.title')" :description="t('docsGuide.sections.cli.description')" />
              <div class="docs-example-controls">
                <label for="docs-client-os">{{ t('docsGuide.controls.operatingSystem') }}</label>
                <select id="docs-client-os" v-model="selectedOS"><option value="unix">macOS / Linux</option><option value="windows">Windows PowerShell</option></select>
              </div>

              <div id="environment-check" class="docs-client-section scroll-mt-24">
                <ClientHeading badge="00" icon="terminal" :title="t('docsGuide.sections.cli.articles.env.title')" />
                <p>{{ t('docsGuide.sections.cli.articles.env.description') }}</p>
                <ul class="docs-check-list"><li v-for="item in translationList('docsGuide.sections.cli.articles.env.items')" :key="item"><Icon name="checkCircle" size="sm" />{{ item }}</li></ul>
                <CodeBlock id="env-check" :label="terminalLabel" :code="environmentCheck" :copied="copiedId === 'env-check'" @copy="copySnippet('env-check', environmentCheck)" />
                <p class="docs-callout">{{ t('docsGuide.sections.cli.articles.env.note') }}</p>
                <p class="docs-source-links"><a href="https://code.claude.com/docs/en/setup" target="_blank" rel="noopener noreferrer">{{ t('docsGuide.sources.claudeInstall') }}<Icon name="externalLink" size="xs" /></a><a href="https://developers.openai.com/codex/cli" target="_blank" rel="noopener noreferrer">{{ t('docsGuide.sources.codexInstall') }}<Icon name="externalLink" size="xs" /></a></p>
              </div>

              <div id="claude-code" class="docs-client-section scroll-mt-24">
                <ClientHeading badge="01" icon="chat" :title="t('docsGuide.sections.cli.articles.claude.title')" />
                <p>{{ t('docsGuide.sections.cli.articles.claude.description') }}</p>
                <NumberedList :items="translationList('docsGuide.sections.cli.articles.claude.items')" />
                <CodeBlock id="claude-settings" :label="claudeSettingsPath" :code="claudeSettingsExample" :copied="copiedId === 'claude-settings'" @copy="copySnippet('claude-settings', claudeSettingsExample)" />
                <p class="docs-callout">{{ t('docsGuide.sections.cli.articles.claude.note') }}</p>
              </div>

              <div id="claude-vscode" class="docs-client-section scroll-mt-24">
                <ClientHeading badge="VS" icon="terminal" :title="t('docsGuide.sections.cli.articles.vscode.title')" />
                <p>{{ t('docsGuide.sections.cli.articles.vscode.description') }}</p>
                <NumberedList :items="translationList('docsGuide.sections.cli.articles.vscode.items')" />
                <CodeBlock id="vscode-settings" label="VS Code · settings.json" :code="vscodeSettingsExample" :copied="copiedId === 'vscode-settings'" @copy="copySnippet('vscode-settings', vscodeSettingsExample)" />
                <p class="docs-callout">{{ t('docsGuide.sections.cli.articles.vscode.note') }}</p>
                <p class="docs-source-links"><a href="https://code.claude.com/docs/en/llm-gateway-connect" target="_blank" rel="noopener noreferrer">{{ t('docsGuide.sources.claudeGateway') }}<Icon name="externalLink" size="xs" /></a></p>
              </div>

              <div id="codex-cli" class="docs-client-section scroll-mt-24">
                <ClientHeading badge="02" icon="cpu" :title="t('docsGuide.sections.cli.articles.codex.title')" />
                <p>{{ t('docsGuide.sections.cli.articles.codex.description') }}</p>
                <NumberedList :items="translationList('docsGuide.sections.cli.articles.codex.items')" />
                <div class="grid gap-4">
                  <CodeBlock id="codex-config" :label="codexConfigPath" :code="codexConfigExample" :copied="copiedId === 'codex-config'" @copy="copySnippet('codex-config', codexConfigExample)" />
                  <CodeBlock id="codex-auth" :label="codexAuthPath" :code="codexAuthExample" :copied="copiedId === 'codex-auth'" @copy="copySnippet('codex-auth', codexAuthExample)" />
                </div>
                <p class="docs-callout">{{ t('docsGuide.sections.cli.articles.codex.note') }}</p>
                <p class="docs-source-links"><a href="https://developers.openai.com/codex/config-reference" target="_blank" rel="noopener noreferrer">{{ t('docsGuide.sources.codexConfig') }}<Icon name="externalLink" size="xs" /></a></p>
              </div>
            </section>

            <section id="desktop-and-switch" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.sections.advanced.eyebrow')" :title="t('docsGuide.sections.advanced.title')" :description="t('docsGuide.sections.advanced.description')" />
              <div class="docs-prose-grid">
                <DocTopic id="claude-desktop" badge="01" icon="cloud" :title="t('docsGuide.sections.advanced.articles.desktop.title')" :description="t('docsGuide.sections.advanced.articles.desktop.description')" :items="translationList('docsGuide.sections.advanced.articles.desktop.items')" :note="t('docsGuide.sections.advanced.articles.desktop.note')">
                  <dl class="docs-settings-list">
                    <div><dt>Gateway base URL</dt><dd>{{ apiBaseUrl }}</dd></div>
                    <div><dt>Gateway auth scheme</dt><dd>x-api-key</dd></div>
                    <div><dt>Gateway API key</dt><dd>sk-...</dd></div>
                  </dl>
                </DocTopic>
                <DocTopic id="cc-switch" badge="02" icon="sync" :title="t('docsGuide.sections.cli.articles.ccSwitch.title')" :description="t('docsGuide.sections.cli.articles.ccSwitch.description')" :items="translationList('docsGuide.sections.cli.articles.ccSwitch.items')" :note="t('docsGuide.sections.cli.articles.ccSwitch.note')" />
              </div>
            </section>

            <section id="faq" class="docs-section scroll-mt-24">
              <SectionHeading :eyebrow="t('docsGuide.sections.faq.eyebrow')" :title="t('docsGuide.sections.faq.title')" :description="t('docsGuide.sections.faq.description')" />
              <div class="docs-faq-list">
                <details v-for="faq in faqItems" :key="faq.title">
                  <summary><span>{{ faq.badge }}</span>{{ faq.title }}<Icon name="chevronDown" size="sm" /></summary>
                  <div><p>{{ faq.description }}</p><ol><li v-for="item in faq.items" :key="item">{{ item }}</li></ol><p class="docs-callout">{{ faq.note }}</p></div>
                </details>
              </div>
            </section>

            <section class="docs-finish">
              <div><p class="docs-kicker">{{ t('docsGuide.bottom.eyebrow') }}</p><h2>{{ t('docsGuide.bottom.title') }}</h2><p>{{ t('docsGuide.bottom.description') }}</p></div>
              <RouterLink :to="isAuthenticated ? dashboardPath : '/register'" class="btn btn-primary px-5 py-2.5 text-sm">{{ isAuthenticated ? t('home.goToDashboard') : t('docsGuide.bottom.button') }}<Icon name="arrowRight" size="sm" /></RouterLink>
            </section>
          </article>
        </div>
      </div>
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, ref, type PropType } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PublicLayout from '@/components/public/PublicLayout.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { normalizeSiteName } from '@/utils/siteBrand'
import { buildGuideRequest, normalizeGuideBaseUrl, type GuideOS, type GuideProtocol } from '@/utils/docsGuideExamples'

type GuideIconName = InstanceType<typeof Icon>['$props']['name']
interface DocsNavLink { id: string; badge: string; title: string }
interface DocsNavGroup { title: string; links: DocsNavLink[] }
interface QuickStartStep { id: string; step: string; title: string; description: string; items: string[] }
interface FAQItem { badge: string; title: string; description: string; items: string[]; note: string }

const SectionHeading = defineComponent({
  props: { eyebrow: { type: String, required: true }, title: { type: String, required: true }, description: { type: String, required: true } },
  setup: (props) => () => h('div', { class: 'docs-section-heading' }, [h('p', { class: 'docs-kicker' }, props.eyebrow), h('h2', props.title), h('p', props.description)])
})

const DocTopic = defineComponent({
  props: {
    id: { type: String, required: true }, badge: { type: String, required: true }, icon: { type: String as PropType<GuideIconName>, required: true },
    title: { type: String, required: true }, description: { type: String, required: true }, items: { type: Array as PropType<string[]>, required: true }, note: { type: String, required: true }
  },
  setup(props, { slots }) {
    return () => h('section', { id: props.id, class: 'docs-topic scroll-mt-24' }, [
      h('div', { class: 'docs-topic-heading' }, [h('div', { class: 'docs-topic-icon' }, [h(Icon, { name: props.icon, size: 'sm' })]), h('div', [h('span', props.badge), h('h3', props.title)])]),
      h('p', { class: 'docs-topic-description' }, props.description),
      h('ul', { class: 'docs-check-list' }, props.items.map((item) => h('li', { key: item }, [h(Icon, { name: 'checkCircle', size: 'sm' }), item]))),
      slots.default?.(), h('p', { class: 'docs-callout' }, props.note)
    ])
  }
})

const CodeBlock = defineComponent({
  emits: ['copy'],
  props: { id: { type: String, required: true }, label: { type: String, required: true }, code: { type: String, required: true }, copied: { type: Boolean, default: false } },
  setup(props, { emit }) {
    const { t } = useI18n()
    return () => h('div', { class: 'docs-code-block', 'data-snippet': props.id }, [
      h('div', { class: 'docs-code-header' }, [h('span', props.label), h('button', { type: 'button', title: t(props.copied ? 'common.copied' : 'common.copy'), 'aria-label': t('docsGuide.controls.copySnippet', { label: props.label }), onClick: () => emit('copy') }, [h(Icon, { name: props.copied ? 'check' : 'copy', size: 'xs' }), props.copied ? t('common.copied') : t('common.copy')])]),
      h('pre', { tabindex: 0, 'aria-label': props.label }, [h('code', props.code)])
    ])
  }
})

const ClientHeading = defineComponent({
  props: { badge: { type: String, required: true }, icon: { type: String as PropType<GuideIconName>, required: true }, title: { type: String, required: true } },
  setup: (props) => () => h('div', { class: 'docs-client-heading' }, [h(Icon, { name: props.icon, size: 'md' }), h('div', [h('span', props.badge), h('h3', props.title)])])
})

const NumberedList = defineComponent({
  props: { items: { type: Array as PropType<string[]>, required: true } },
  setup: (props) => () => h('ol', { class: 'docs-numbered-list' }, props.items.map((item, index) => h('li', { key: item }, [h('span', String(index + 1)), item])))
})

const { t, tm } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const isDark = ref(document.documentElement.classList.contains('dark'))
const selectedOS = ref<GuideOS>('unix')
const selectedProtocol = ref<GuideProtocol>('messages')
const modelOverrides = ref<Partial<Record<GuideProtocol, string>>>({})
const copiedId = ref('')
let copyTimer: number | undefined
const themeLabel = computed(() => t(isDark.value ? 'docsGuide.controls.lightMode' : 'docsGuide.controls.darkMode'))
const terminalLabel = computed(() => selectedOS.value === 'windows' ? 'PowerShell' : 'Bash / Zsh')

const siteName = computed(() => normalizeSiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const apiBaseUrl = computed(() => normalizeGuideBaseUrl(appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || 'https://api.subapis.com'))
const usageConfig = computed(() => appStore.cachedPublicSettings?.api_key_usage_config)
const codexModel = computed(() => usageConfig.value?.codex_model || 'gpt-5.6-sol')
const codexReviewModel = computed(() => usageConfig.value?.codex_review_model || codexModel.value)
const claudeModel = computed(() => usageConfig.value?.claude_code_default_model || 'claude-sonnet-4-6')
const requestModel = computed({
  get: () => modelOverrides.value[selectedProtocol.value] ?? (selectedProtocol.value === 'messages' ? claudeModel.value : codexModel.value),
  set: (value: string) => { modelOverrides.value[selectedProtocol.value] = value }
})

const docsNavigation = computed<DocsNavGroup[]>(() => [
  { title: t('docsGuide.navGroups.quickStart'), links: [
    { id: 'quick-start', badge: '01', title: t('docsGuide.quickStart.title') }, { id: 'keys-and-models', badge: '02', title: t('docsGuide.sections.models.title') }, { id: 'endpoint-map', badge: '03', title: t('docsGuide.examples.title') }
  ] },
  { title: t('docsGuide.navGroups.cli'), links: [
    { id: 'environment-check', badge: '00', title: t('docsGuide.sections.cli.articles.env.title') }, { id: 'claude-code', badge: 'CC', title: t('docsGuide.sections.cli.articles.claude.title') }, { id: 'claude-vscode', badge: 'VS', title: t('docsGuide.sections.cli.articles.vscode.title') }, { id: 'codex-cli', badge: 'CX', title: t('docsGuide.sections.cli.articles.codex.title') }
  ] },
  { title: t('docsGuide.navGroups.more'), links: [
    { id: 'claude-desktop', badge: 'CD', title: t('docsGuide.sections.advanced.articles.desktop.title') }, { id: 'cc-switch', badge: 'SW', title: t('docsGuide.sections.cli.articles.ccSwitch.title') }, { id: 'faq', badge: 'FAQ', title: t('docsGuide.sections.faq.title') }
  ] }
])

const quickStartSteps = computed<QuickStartStep[]>(() => [
  createStep('register', '01', 'docsGuide.steps.register'), createStep('login', '02', 'docsGuide.steps.login'), createStep('billing', '03', 'docsGuide.steps.billing'),
  createStep('token', '04', 'docsGuide.steps.token'), createStep('environment', '05', 'docsGuide.steps.environment'), createStep('first-call', '06', 'docsGuide.steps.firstCall')
])

const endpoints = computed(() => [
  { name: 'Anthropic Messages', path: `${apiBaseUrl.value}/v1/messages` }, { name: 'OpenAI Responses', path: `${apiBaseUrl.value}/v1/responses` },
  { name: 'OpenAI Chat Completions', path: `${apiBaseUrl.value}/v1/chat/completions` }, { name: 'Model list', path: `${apiBaseUrl.value}/v1/models` }
])

const curlExample = computed(() => buildGuideRequest(apiBaseUrl.value, requestModel.value.trim(), selectedProtocol.value, selectedOS.value))

const environmentCheck = computed(() => selectedOS.value === 'windows'
  ? `curl.exe -fsS "${apiBaseUrl.value}/health"`
  : `curl -fsS "${apiBaseUrl.value}/health"`)
const claudeSettingsPath = computed(() => selectedOS.value === 'windows' ? '%USERPROFILE%\\.claude\\settings.json' : '~/.claude/settings.json')
const claudeSettingsExample = computed(() => JSON.stringify({ env: {
  ANTHROPIC_BASE_URL: apiBaseUrl.value, ANTHROPIC_AUTH_TOKEN: 'sk-your-key', CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1', CLAUDE_CODE_ATTRIBUTION_HEADER: String(usageConfig.value?.claude_code_attribution_header ?? 0)
} }, null, 2))
const vscodeSettingsExample = computed(() => JSON.stringify({ 'claudeCode.environmentVariables': [
  { name: 'ANTHROPIC_BASE_URL', value: apiBaseUrl.value },
  { name: 'ANTHROPIC_AUTH_TOKEN', value: 'sk-your-key' }
] }, null, 2))
const codexConfigPath = computed(() => selectedOS.value === 'windows' ? '%USERPROFILE%\\.codex\\config.toml' : '~/.codex/config.toml')
const codexAuthPath = computed(() => selectedOS.value === 'windows' ? '%USERPROFILE%\\.codex\\auth.json' : '~/.codex/auth.json')
const codexConfigExample = computed(() => {
  const lines = ['model_provider = "OpenAI"', `model = ${JSON.stringify(codexModel.value)}`, `review_model = ${JSON.stringify(codexReviewModel.value)}`,
    `model_reasoning_effort = ${JSON.stringify(usageConfig.value?.codex_reasoning_effort || 'xhigh')}`,
    'cli_auth_credentials_store = "file"', '', '[model_providers.OpenAI]', 'name = "OpenAI"',
    `base_url = ${JSON.stringify(apiBaseUrl.value)}`, 'wire_api = "responses"']
  // The public guide is the HTTP/SSE baseline. Group-specific WebSocket and
  // model-catalog settings are provided by the authenticated Use Key generator.
  lines.push('supports_websockets = false')
  lines.push('requires_openai_auth = true')
  return lines.join('\n')
})
const codexAuthExample = '{\n  "OPENAI_API_KEY": "sk-your-key"\n}'
const faqItems = computed<FAQItem[]>(() => [createFAQ('Q1', 'docsGuide.sections.faq.articles.noModel'), createFAQ('Q2', 'docsGuide.sections.faq.articles.auth'), createFAQ('Q3', 'docsGuide.sections.faq.articles.billing'), createFAQ('Q4', 'docsGuide.sections.faq.articles.latency')])

function createStep(id: string, step: string, key: string): QuickStartStep { return { id, step, title: t(`${key}.title`), description: t(`${key}.description`), items: translationList(`${key}.items`) } }
function createFAQ(badge: string, key: string): FAQItem { return { badge, title: t(`${key}.title`), description: t(`${key}.description`), items: translationList(`${key}.items`), note: t(`${key}.note`) } }
function translationList(key: string): string[] { const value = tm(key); return Array.isArray(value) ? value.map(String) : [] }
async function copySnippet(id: string, value: string) {
  if (!await copyToClipboard(value)) return
  if (copyTimer) window.clearTimeout(copyTimer)
  copiedId.value = id
  copyTimer = window.setTimeout(() => { copiedId.value = '' }, 1600)
}
function closeMobileNav(event: MouseEvent) { (event.currentTarget as HTMLElement).closest('details')?.removeAttribute('open') }
function toggleTheme() { isDark.value = !isDark.value; document.documentElement.classList.toggle('dark', isDark.value); localStorage.setItem('theme', isDark.value ? 'dark' : 'light') }
onUnmounted(() => { if (copyTimer) window.clearTimeout(copyTimer) })

onMounted(() => {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) { isDark.value = true; document.documentElement.classList.add('dark') }
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
</script>


<style>
/* Render-function children do not inherit the SFC scope attribute. Keep every
   selector under .docs-page instead, so the guide cannot restyle other routes. */
.docs-page { --docs-border: #e5e7eb; --docs-muted: #64748b; }
.dark .docs-page { --docs-border: #2f3642; --docs-muted: #9ca3af; }
.docs-page .docs-topbar { background: rgba(255,255,255,.94); border-bottom: 1px solid var(--docs-border); position: sticky; top: 0; z-index: 30; }
.dark .docs-page .docs-topbar { background: rgba(17,21,28,.94); }
.docs-page .docs-topbar-inner { align-items: center; display: flex; height: 3.75rem; justify-content: space-between; margin: 0 auto; max-width: 90rem; padding: 0 1.25rem; }
.docs-page .docs-brand { align-items: center; display: flex; font-size: .94rem; font-weight: 800; gap: .6rem; min-width: 0; }
.docs-page .docs-brand-mark { align-items: center; background: #111827; border-radius: 6px; color: #fff; display: flex; height: 1.75rem; justify-content: center; width: 1.75rem; }
.dark .docs-page .docs-brand-mark { background: #14b8a6; color: #071a18; }
.docs-page .docs-brand-divider { background: var(--docs-border); height: 1.25rem; width: 1px; }
.docs-page .docs-icon-link,.docs-page .docs-icon-button { align-items: center; border-radius: 6px; color: #4b5563; display: inline-flex; font-size: .875rem; font-weight: 650; gap: .4rem; min-height: 2rem; padding: 0 .55rem; }
.docs-page .docs-icon-link:hover,.docs-page .docs-icon-button:hover { background: #f3f4f6; color: #111827; }
.dark .docs-page .docs-icon-link,.dark .docs-page .docs-icon-button { color: #c4c9d2; }
.dark .docs-page .docs-icon-link:hover,.dark .docs-page .docs-icon-button:hover { background: #262c35; color: #fff; }
.docs-page .docs-main { margin: 0 auto; max-width: 90rem; padding: 0 1.25rem 4rem; }
.docs-page .docs-intro { align-items: end; border-bottom: 1px solid var(--docs-border); display: grid; gap: 2rem; grid-template-columns: minmax(0,1fr) minmax(18rem,28rem); padding: 3.25rem 0 2.5rem; }
.docs-page .docs-kicker { color: #0f766e; font-size: .72rem; font-weight: 800; letter-spacing: 0; text-transform: uppercase; }
.dark .docs-page .docs-kicker { color: #5eead4; }
.docs-page .docs-intro h1 { font-size: 2.5rem; font-weight: 850; letter-spacing: 0; line-height: 1.12; margin-top: .75rem; max-width: 52rem; }
.docs-page .docs-lead { color: var(--docs-muted); font-size: 1rem; line-height: 1.75; margin-top: 1rem; max-width: 48rem; }
.docs-page .docs-base-url { align-items: center; background: #f8fafc; border: 1px solid var(--docs-border); border-radius: 8px; display: grid; gap: .4rem .75rem; grid-template-columns: minmax(0,1fr) auto; padding: .9rem 1rem; }
.dark .docs-page .docs-base-url { background: #151a22; }
.docs-page .docs-base-url>span { color: var(--docs-muted); font-size: .7rem; font-weight: 750; grid-column: 1/-1; }
.docs-page .docs-base-url code { font-size: .875rem; overflow-wrap: anywhere; }
.docs-page .docs-base-url button { color: #0f766e; }
.docs-page .docs-layout { display: grid; gap: 3.5rem; grid-template-columns: 15rem minmax(0,1fr); }
.docs-page .docs-sidebar { align-self: start; max-height: calc(100vh - 5rem); overflow-y: auto; padding: 2.25rem 0; position: sticky; top: 3.75rem; }
.docs-page .docs-nav-group+.docs-nav-group { margin-top: 1.5rem; }
.docs-page .docs-nav-group>p { color: #111827; font-size: .72rem; font-weight: 800; margin-bottom: .45rem; }
.dark .docs-page .docs-nav-group>p { color: #f3f4f6; }
.docs-page .docs-nav-group a { align-items: center; border-radius: 5px; color: var(--docs-muted); display: flex; font-size: .875rem; gap: .55rem; line-height: 1.35; padding: .4rem .5rem; }
.docs-page .docs-nav-group a:hover { background: #f3f4f6; color: #0f766e; }
.dark .docs-page .docs-nav-group a:hover { background: #202630; color: #5eead4; }
.docs-page .docs-nav-group a span { color: #94a3b8; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .63rem; min-width: 1.8rem; }
.docs-page .docs-content { border-left: 1px solid var(--docs-border); min-width: 0; padding-left: 3.5rem; }
.docs-page .docs-section { border-bottom: 1px solid var(--docs-border); padding: 3rem 0; }
.docs-page .docs-section-heading { max-width: 50rem; }
.docs-page .docs-section-heading h2 { font-size: 1.75rem; font-weight: 820; letter-spacing: 0; line-height: 1.25; margin-top: .6rem; }
.docs-page .docs-section-heading>p:last-child { color: var(--docs-muted); font-size: .92rem; line-height: 1.75; margin-top: .75rem; }
.docs-page .docs-steps { margin-top: 2rem; }
.docs-page .docs-steps>li { display: grid; gap: 1.25rem; grid-template-columns: 2rem minmax(0,1fr); padding: 1.5rem 0; }
.docs-page .docs-steps>li+li { border-top: 1px solid var(--docs-border); }
.docs-page .docs-step-number { align-items: center; background: #ecfdf5; border-radius: 6px; color: #047857; display: flex; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .7rem; font-weight: 800; height: 2rem; justify-content: center; width: 2rem; }
.dark .docs-page .docs-step-number { background: rgba(20,184,166,.14); color: #5eead4; }
.docs-page .docs-steps h3 { font-size: 1rem; font-weight: 800; }
.docs-page .docs-steps p { color: var(--docs-muted); font-size: .92rem; line-height: 1.65; margin-top: .4rem; }
.docs-page .docs-steps ul { display: grid; gap: .45rem 1.5rem; grid-template-columns: repeat(2,minmax(0,1fr)); margin-top: .85rem; }
.docs-page .docs-steps ul li,.docs-page .docs-check-list li { align-items: start; color: #374151; display: flex; font-size: .875rem; gap: .5rem; line-height: 1.55; }
.dark .docs-page .docs-steps ul li,.dark .docs-page .docs-check-list li { color: #d1d5db; }
.docs-page .docs-steps ul svg,.docs-page .docs-check-list svg { color: #0d9488; flex: none; margin-top: .1rem; }
.docs-page .docs-prose-grid { display: grid; gap: 0 2.5rem; grid-template-columns: repeat(2,minmax(0,1fr)); margin-top: 1.5rem; }
.docs-page .docs-topic { border-top: 1px solid var(--docs-border); padding: 1.75rem 0; }
.docs-page .docs-topic-heading,.docs-page .docs-client-heading { align-items: center; display: flex; gap: .8rem; }
.docs-page .docs-topic-icon { align-items: center; background: #f0fdfa; border-radius: 6px; color: #0f766e; display: flex; height: 2.25rem; justify-content: center; width: 2.25rem; }
.dark .docs-page .docs-topic-icon { background: rgba(20,184,166,.12); color: #5eead4; }
.docs-page .docs-topic-heading span,.docs-page .docs-client-heading span { color: #94a3b8; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .65rem; }
.docs-page .docs-topic-heading h3,.docs-page .docs-client-heading h3 { font-size: 1rem; font-weight: 800; margin-top: .1rem; }
.docs-page .docs-topic-description,.docs-page .docs-client-section>p { color: var(--docs-muted); font-size: .92rem; line-height: 1.7; margin-top: 1rem; }
.docs-page .docs-check-list { display: grid; gap: .55rem; margin-top: 1rem; }
.docs-page .docs-callout { background: #f8fafc; border-left: 3px solid #14b8a6; border-radius: 0 5px 5px 0; color: #475569; font-size: .76rem; line-height: 1.6; margin-top: 1rem; padding: .7rem .85rem; }
.dark .docs-page .docs-callout { background: #171c24; color: #c4c9d2; }
.docs-page .docs-inline-link { align-items: center; color: #0f766e; display: inline-flex; font-size: .875rem; font-weight: 750; gap: .35rem; margin-top: 1rem; }
.dark .docs-page .docs-inline-link { color: #5eead4; }
.docs-page .docs-endpoint-table,.docs-page .docs-settings-list { border: 1px solid var(--docs-border); border-radius: 8px; margin-top: 1.5rem; overflow: hidden; }
.docs-page .docs-endpoint-table>div { align-items: center; display: grid; gap: 1rem; grid-template-columns: 12rem minmax(0,1fr); padding: .8rem 1rem; }
.docs-page .docs-endpoint-table>div+div,.docs-page .docs-settings-list>div+div { border-top: 1px solid var(--docs-border); }
.docs-page .docs-endpoint-table span { color: #374151; font-size: .875rem; font-weight: 700; }
.dark .docs-page .docs-endpoint-table span { color: #e5e7eb; }
.docs-page .docs-endpoint-table code { color: var(--docs-muted); font-size: .8125rem; min-width: 0; overflow-wrap: anywhere; white-space: normal; }
.docs-page .docs-code-block { background: #111827; border: 1px solid #253047; border-radius: 8px; margin-top: 1.25rem; min-width: 0; overflow: hidden; }
.docs-page .docs-code-header { align-items: center; background: #182132; border-bottom: 1px solid #2b3649; color: #a8b3c5; display: flex; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .75rem; gap: .75rem; justify-content: space-between; min-height: 2.25rem; padding: 0 .8rem; }
.docs-page .docs-code-header button { align-items: center; border-radius: 5px; color: #cbd5e1; display: flex; gap: .4rem; min-height: 2.25rem; justify-content: center; padding: 0 .6rem; }
.docs-page .docs-code-header button:hover { background: #2b3649; color: #fff; }
.docs-page .docs-code-block pre { color: #e5edf8; font-size: .8125rem; line-height: 1.65; max-height: 28rem; overflow: auto; padding: 1rem; white-space: pre; }
.docs-page .docs-segmented { margin-top: 0; background: #f1f5f9; border-radius: 7px; display: inline-flex; padding: .2rem; }
.dark .docs-page .docs-segmented { margin-top: 0; background: #202630; }
.docs-page .docs-segmented label { align-items: center; cursor: pointer; display: inline-flex; position: relative; border-radius: 5px; color: var(--docs-muted); font-size: .72rem; font-weight: 750; min-height: 2.5rem; padding: 0 .85rem; }
.docs-page .docs-segmented label.active { background: #fff; color: #111827; box-shadow: 0 1px 2px rgba(15,23,42,.12); }
.dark .docs-page .docs-segmented label.active { background: #343c49; color: #fff; }
.docs-page .docs-client-section { padding: 2rem 0 0; }
.docs-page .docs-client-section+.docs-client-section { border-top: 1px solid var(--docs-border); margin-top: 2rem; }
.docs-page .docs-client-heading>svg { color: #0f766e; }
.dark .docs-page .docs-client-heading>svg { color: #5eead4; }
.docs-page .docs-numbered-list { display: grid; gap: .65rem; margin-top: 1rem; }
.docs-page .docs-numbered-list li { align-items: start; color: #374151; display: flex; font-size: .875rem; gap: .65rem; line-height: 1.55; }
.dark .docs-page .docs-numbered-list li { color: #d1d5db; }
.docs-page .docs-numbered-list li>span { align-items: center; border: 1px solid var(--docs-border); border-radius: 50%; color: #0f766e; display: flex; flex: none; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .62rem; height: 1.35rem; justify-content: center; margin-top: .04rem; width: 1.35rem; }
.docs-page .docs-settings-list>div { display: grid; gap: .75rem; grid-template-columns: 10rem minmax(0,1fr); padding: .65rem .8rem; }
.docs-page .docs-settings-list dt { color: var(--docs-muted); font-size: .7rem; }
.docs-page .docs-settings-list dd { font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .72rem; overflow-wrap: anywhere; }
.docs-page .docs-faq-list { border-top: 1px solid var(--docs-border); margin-top: 1.5rem; }
.docs-page .docs-faq-list details { border-bottom: 1px solid var(--docs-border); }
.docs-page .docs-faq-list summary { align-items: center; cursor: pointer; display: grid; font-size: .9rem; font-weight: 750; gap: .75rem; grid-template-columns: 2rem minmax(0,1fr) auto; list-style: none; padding: 1rem 0; }
.docs-page .docs-faq-list summary::-webkit-details-marker { display: none; }
.docs-page .docs-faq-list summary>span { color: #0f766e; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: .65rem; }
.docs-page .docs-faq-list details[open] summary svg { transform: rotate(180deg); }
.docs-page .docs-faq-list details>div { padding: 0 0 1.25rem 2.75rem; }
.docs-page .docs-faq-list details>div>p:first-child { color: var(--docs-muted); font-size: .82rem; line-height: 1.65; }
.docs-page .docs-faq-list ol { color: #374151; font-size: .875rem; line-height: 1.6; list-style: decimal; margin: .8rem 0 0 1rem; }
.dark .docs-page .docs-faq-list ol { color: #d1d5db; }
.docs-page .docs-finish { align-items: center; display: flex; gap: 2rem; justify-content: space-between; padding: 3rem 0 0; }
.docs-page .docs-finish h2 { font-size: 1.35rem; font-weight: 820; margin-top: .45rem; }
.docs-page .docs-finish>div>p:last-child { color: var(--docs-muted); font-size: .82rem; line-height: 1.65; margin-top: .5rem; max-width: 42rem; }
@media (max-width:1023px) {
.docs-page .docs-intro { grid-template-columns: 1fr; }
.docs-page .docs-layout { display: block; }
.docs-page .docs-sidebar { display: none; }
.docs-page .docs-content { border-left: 0; padding-left: 0; } }
@media (max-width:639px) {
.docs-page .docs-topbar-inner,.docs-page .docs-main { padding-left: 1rem; padding-right: 1rem; }
.docs-page .docs-brand-divider,.docs-page .docs-brand>span:last-child { display: none; }
.docs-page .docs-intro { padding: 2.25rem 0 1.75rem; }
.docs-page .docs-intro h1 { font-size: 2rem; }
.docs-page .docs-section { padding: 2.25rem 0; }
.docs-page .docs-prose-grid,.docs-page .docs-steps ul { grid-template-columns: 1fr; }
.docs-page .docs-endpoint-table>div { align-items: start; gap: .3rem; grid-template-columns: 1fr; }
.docs-page .docs-settings-list>div { gap: .25rem; grid-template-columns: 1fr; }
.docs-page .docs-faq-list details>div { padding-left: 0; }
.docs-page .docs-finish { align-items: stretch; flex-direction: column; }
}
@media (prefers-reduced-motion:reduce) {
.docs-page *,.docs-page *::before,.docs-page *::after { scroll-behavior: auto !important; transition: none !important; } }

.docs-page .docs-os-picker { border-bottom: 1px solid var(--docs-border); padding: 1.25rem 0; }
.docs-page .docs-os-picker legend { float: left; font-size: .875rem; font-weight: 700; margin: .7rem 1rem .7rem 0; }
.docs-page .docs-os-picker p { clear: both; color: var(--docs-muted); font-size: .8rem; margin-top: .6rem; }
.docs-page .docs-segmented input { height: 1px; opacity: 0; position: absolute; width: 1px; }
.docs-page .docs-segmented label:has(input:focus-visible) { outline: 2px solid #0d9488; outline-offset: 3px; }
.docs-page .docs-example-controls { display: grid; gap: .6rem; grid-template-columns: auto minmax(0,1fr); margin-top: 1.5rem; align-items: center; }
.docs-page .docs-example-controls label { font-size: .875rem; font-weight: 700; }
.docs-page .docs-example-controls select,.docs-page .docs-example-controls input { background: transparent; border: 1px solid var(--docs-border); border-radius: 6px; color: inherit; font-size: .875rem; min-width: 0; padding: .65rem .8rem; width: 100%; }
.dark .docs-page .docs-example-controls option { background: #151a22; }
.docs-page .docs-source-links { display: flex; flex-wrap: wrap; gap: .8rem 1.5rem; margin-top: 1rem; }
.docs-page .docs-source-links a { align-items: center; color: #0f766e; display: inline-flex; font-size: .875rem; gap: .35rem; }
.dark .docs-page .docs-source-links a { color: #5eead4; }
.docs-page .docs-code-header>span { overflow-wrap: anywhere; min-width: 0; }
.docs-page .docs-code-header button { flex: none; }
.docs-page .docs-prose-grid>.docs-topic:last-child:nth-child(odd) { grid-column: 1/-1; }
.docs-page .docs-topic-description { max-width: 70ch; }
.docs-page .docs-mobile-nav { display: none; }
.docs-page :is(a,button,select,input,summary,pre):focus-visible { outline: 2px solid #0d9488; outline-offset: 3px; }
.docs-page :is(button,a,label):active { opacity: .8; }
@media (max-width:1023px) {
  .docs-page .docs-mobile-nav { border-bottom: 1px solid var(--docs-border); display: block; padding: .8rem 0; }
  .docs-page .docs-mobile-nav summary { align-items: center; cursor: pointer; display: flex; font-size: .875rem; font-weight: 700; justify-content: space-between; list-style: none; min-height: 2.5rem; }
  .docs-page .docs-mobile-nav summary::-webkit-details-marker { display: none; }
  .docs-page .docs-mobile-nav nav { display: grid; gap: .35rem; padding: .75rem 0; }
  .docs-page .docs-mobile-nav a { color: var(--docs-muted); font-size: .875rem; padding: .7rem .5rem; }
}
@media (max-width:639px) {
  .docs-page .docs-example-controls { grid-template-columns: 1fr; }
  .docs-page .docs-os-picker legend { float: none; margin-top: .5rem; }
  .docs-page .docs-icon-link,.docs-page .docs-icon-button,.docs-page .docs-topbar nav button { min-height: 2.75rem; }
  .docs-page .docs-base-url button { min-height: 2.75rem; min-width: 2.75rem; }
  .docs-page .docs-code-header button { min-height: 2.75rem; }
  .docs-page .docs-code-header { padding: .5rem .6rem; }
}
</style>
