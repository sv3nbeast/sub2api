import type { RouterScrollBehavior } from 'vue-router'

export const scrollBehavior: RouterScrollBehavior = (to, _from, savedPosition) => {
  if (savedPosition) return savedPosition

  // Native anchors also update Vue Router. Its default top: 0 would undo
  // the browser's jump; keep document headings below the sticky header.
  if (to.path === '/docs' && to.hash) {
    const position = () => document.getElementById(to.hash.slice(1))
      ? { el: to.hash, top: 96 }
      : { top: 0 }
    if (document.getElementById(to.hash.slice(1))) return position()

    // main.ts mounts after router.isReady(). On a direct section URL the
    // first scroll can precede that mount; the next frame sees the heading.
    return new Promise(resolve => requestAnimationFrame(() => resolve(position())))
  }

  return { top: 0 }
}
