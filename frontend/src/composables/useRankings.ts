import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { getRankings, type RankingPeriod, type RankingsSnapshot } from '@/api/rankings'

// Cancel the network request and guard its completion: even an adapter ignoring
// AbortSignal must never replace a newer period's data or loading/error state.
export function useRankings(period: Ref<RankingPeriod>) {
  const snapshot = ref<RankingsSnapshot | null>(null)
  const loading = ref(false)
  const error = ref(false)
  let generation = 0
  let controller: AbortController | null = null

  async function reload() {
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = false
    snapshot.value = null

    try {
      const response = await getRankings(period.value, { signal: controller.signal })
      if (current === generation) snapshot.value = response
    } catch {
      if (current === generation) error.value = true
    } finally {
      if (current === generation) loading.value = false
    }
  }

  watch(period, reload, { immediate: true })
  onBeforeUnmount(() => {
    generation++
    controller?.abort()
  })

  return { snapshot, loading, error, reload }
}
