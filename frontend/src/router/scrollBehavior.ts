import type { RouterScrollBehavior } from 'vue-router'

export const scrollBehavior: RouterScrollBehavior = (to, _from, savedPosition) => {
  if (savedPosition) return savedPosition

  // Native anchors also update Vue Router. Its default top: 0 would undo
  // the browser's jump; keep document headings below the sticky header.
  if (to.path === '/docs' && to.hash && document.getElementById(to.hash.slice(1))) {
    return { el: to.hash, top: 96 }
  }

  return { top: 0 }
}
