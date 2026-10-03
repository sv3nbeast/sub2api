import { flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter, type RouterHistory } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { scrollBehavior } from '../scrollBehavior'

const histories: RouterHistory[] = []

function createTestRouter() {
  const history = createMemoryHistory()
  histories.push(history)
  return createRouter({
    history,
    routes: [
      { path: '/docs', component: { template: '<div />' } },
      { path: '/dashboard', component: { template: '<div />' } }
    ],
    scrollBehavior
  })
}

afterEach(() => {
  histories.splice(0).forEach(history => history.destroy())
  document.getElementById('claude-code')?.remove()
  vi.restoreAllMocks()
})

describe('documentation anchor navigation', () => {
  it('lets Vue Router finish scrolling to a document heading instead of returning to the page top', async () => {
    const section = document.createElement('section')
    section.id = 'claude-code'
    document.body.append(section)
    vi.spyOn(section, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 1600, 100, 100))
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const router = createTestRouter()

    await router.push('/docs')
    await flushPromises()
    await router.push('/docs#claude-code')
    await flushPromises()

    expect(router.currentRoute.value.hash).toBe('#claude-code')
    expect(scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ top: 1504 }))
  })

  it('keeps normal routes and missing document anchors at the page top', async () => {
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => {
      queueMicrotask(() => callback(0))
      return 1
    })
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const router = createTestRouter()
    for (const path of ['/docs#missing', '/dashboard#claude-code']) {
      await router.push(path)
      await flushPromises()
      expect(scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ top: 0 }))
    }
  })

  it('positions an initial section URL when the page mounts after router.isReady()', async () => {
    let mountFrame: FrameRequestCallback | undefined
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => {
      mountFrame = callback
      return 1
    })
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const router = createTestRouter()
    await router.push('/docs#claude-code')
    await router.isReady()
    await flushPromises()

    const section = document.createElement('section')
    section.id = 'claude-code'
    document.body.append(section)
    vi.spyOn(section, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 1600, 100, 100))
    expect(mountFrame).toBeDefined()
    mountFrame!(0)
    await flushPromises()

    expect(scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ top: 1504 }))
  })

  it('preserves saved positions when returning through browser history', () => {
    const router = createTestRouter()
    expect(scrollBehavior(router.resolve('/docs#claude-code'), router.resolve('/dashboard'), { left: 0, top: 420 }))
      .toEqual({ left: 0, top: 420 })
  })
})
