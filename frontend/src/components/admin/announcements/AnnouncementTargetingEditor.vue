<template>
  <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/50">
    <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.announcements.form.targetingMode') }}
        </div>
        <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
          {{ mode === 'all' ? t('admin.announcements.form.targetingAll') : t('admin.announcements.form.targetingCustom') }}
        </div>
      </div>

      <div class="flex items-center gap-3">
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input
            type="radio"
            name="announcement-targeting-mode"
            value="all"
            :checked="mode === 'all'"
            @change="setMode('all')"
            class="h-4 w-4"
          />
          {{ t('admin.announcements.form.targetingAll') }}
        </label>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input
            type="radio"
            name="announcement-targeting-mode"
            value="custom"
            :checked="mode === 'custom'"
            @change="setMode('custom')"
            class="h-4 w-4"
          />
          {{ t('admin.announcements.form.targetingCustom') }}
        </label>
      </div>
    </div>

    <div v-if="mode === 'custom'" class="mt-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.announcements.form.orGroupLabel') }}
          <span class="ml-1 text-xs font-normal text-gray-500 dark:text-dark-400">
            ({{ anyOf.length }}/50)
          </span>
        </div>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="anyOf.length >= 50"
          @click="addOrGroup"
        >
          <Icon name="plus" size="sm" class="mr-1" />
          {{ t('admin.announcements.form.addOrGroup') }}
        </button>
      </div>

      <div v-if="anyOf.length === 0" class="rounded-xl border border-dashed border-gray-300 p-4 text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
        {{ t('admin.announcements.form.targetingCustom') }}: {{ t('admin.announcements.form.addOrGroup') }}
      </div>

      <div
        v-for="(group, groupIndex) in anyOf"
        :key="groupIndex"
        class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.announcements.form.targetingCustom') }} #{{ groupIndex + 1 }}
              <span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">
                {{ t('admin.announcements.form.andConditionLabel') }} ({{ (group.all_of?.length || 0) }}/50)
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.announcements.form.addAndCondition') }}
            </div>
          </div>

          <button
            type="button"
            class="btn btn-secondary"
            @click="removeOrGroup(groupIndex)"
          >
            <Icon name="trash" size="sm" class="mr-1" />
            {{ t('common.delete') }}
          </button>
        </div>

        <div class="mt-4 space-y-3">
          <div
            v-for="(cond, condIndex) in (group.all_of || [])"
            :key="condIndex"
            class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/30"
          >
            <div class="flex flex-col gap-3 md:flex-row md:items-end">
              <div class="w-full md:w-52">
                <label class="input-label">{{ t('admin.announcements.form.conditionType') }}</label>
                <Select
                  :model-value="cond.type"
                  :options="conditionTypeOptions"
                  @update:model-value="(v) => setConditionType(groupIndex, condIndex, v as any)"
                />
              </div>

              <div v-if="cond.type === 'subscription'" class="flex-1">
                <label class="input-label">{{ t('admin.announcements.form.selectPackages') }}</label>
                <GroupSelector
                  v-model="subscriptionSelections[groupIndex][condIndex]"
                  :groups="groups"
                />
              </div>

              <div v-else-if="cond.type === 'user'" class="flex-1">
                <label class="input-label">{{ t('admin.announcements.form.selectUsers') }}</label>
                <div class="flex flex-col gap-2 sm:flex-row">
                  <input
                    v-model="userTargetState(groupIndex, condIndex).query"
                    type="search"
                    class="input flex-1"
                    :placeholder="t('admin.announcements.form.searchUsersByEmail')"
                    @keydown.enter.prevent="searchTargetUsers(groupIndex, condIndex)"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary shrink-0"
                    :disabled="userTargetState(groupIndex, condIndex).loading"
                    @click="searchTargetUsers(groupIndex, condIndex)"
                  >
                    <Icon
                      name="search"
                      size="sm"
                      class="mr-1"
                      :class="userTargetState(groupIndex, condIndex).loading ? 'animate-spin' : ''"
                    />
                    {{ t('admin.announcements.form.searchUsers') }}
                  </button>
                </div>

                <div
                  v-if="userTargetState(groupIndex, condIndex).searched"
                  class="mt-2 rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
                >
                  <div
                    v-if="userTargetState(groupIndex, condIndex).results.length === 0"
                    class="px-3 py-2 text-xs text-gray-500 dark:text-dark-400"
                  >
                    {{ t('admin.announcements.form.noUsersFound') }}
                  </div>
                  <button
                    v-for="user in userTargetState(groupIndex, condIndex).results"
                    :key="user.id"
                    type="button"
                    class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition-colors hover:bg-gray-50 dark:hover:bg-dark-800"
                    :class="isUserSelected(cond, user.id) ? 'opacity-50' : ''"
                    :disabled="isUserSelected(cond, user.id)"
                    @click="addUserTarget(groupIndex, condIndex, user.id)"
                  >
                    <span class="min-w-0 truncate text-gray-900 dark:text-white">{{ user.email }}</span>
                    <span class="shrink-0 text-xs text-gray-500 dark:text-dark-400">#{{ user.id }}</span>
                  </button>
                </div>

                <div v-if="(cond.user_ids || []).length > 0" class="mt-2 flex flex-wrap gap-2">
                  <span
                    v-for="userId in cond.user_ids"
                    :key="userId"
                    class="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2.5 py-1 text-xs text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
                  >
                    {{ selectedUserLabel(groupIndex, condIndex, userId) }}
                    <button
                      type="button"
                      class="rounded-full p-0.5 hover:bg-blue-100 dark:hover:bg-blue-900/50"
                      :aria-label="t('admin.announcements.form.removeUser')"
                      @click="removeUserTarget(groupIndex, condIndex, userId)"
                    >
                      <Icon name="x" size="xs" />
                    </button>
                  </span>
                </div>
              </div>

              <div v-else class="flex flex-1 flex-col gap-3 sm:flex-row">
                <div class="w-full sm:w-44">
                  <label class="input-label">{{ t('admin.announcements.form.operator') }}</label>
                  <Select
                    :model-value="cond.operator"
                    :options="balanceOperatorOptions"
                    @update:model-value="(v) => setOperator(groupIndex, condIndex, v as any)"
                  />
                </div>
                <div class="w-full sm:flex-1">
                  <label class="input-label">{{ t('admin.announcements.form.balanceValue') }}</label>
                  <input
                    :value="String(cond.value ?? '')"
                    type="number"
                    step="any"
                    class="input"
                    @input="(e) => setBalanceValue(groupIndex, condIndex, (e.target as HTMLInputElement).value)"
                  />
                </div>
              </div>

              <div class="flex justify-end">
                <button
                  type="button"
                  class="btn btn-secondary"
                  @click="removeAndCondition(groupIndex, condIndex)"
                >
                  <Icon name="trash" size="sm" class="mr-1" />
                  {{ t('common.delete') }}
                </button>
              </div>
            </div>
          </div>

          <div class="flex justify-end">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="(group.all_of?.length || 0) >= 50"
              @click="addAndCondition(groupIndex)"
            >
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('admin.announcements.form.addAndCondition') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="validationError" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
        {{ validationError }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SimpleUser } from '@/api/admin/usage'
import type {
  AdminGroup,
  AnnouncementTargeting,
  AnnouncementCondition,
  AnnouncementConditionGroup,
  AnnouncementConditionType,
  AnnouncementOperator
} from '@/types'

import Select from '@/components/common/Select.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const props = defineProps<{
  modelValue: AnnouncementTargeting
  groups: AdminGroup[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: AnnouncementTargeting): void
}>()

const anyOf = computed(() => props.modelValue?.any_of ?? [])

type Mode = 'all' | 'custom'
const mode = computed<Mode>(() => (anyOf.value.length === 0 ? 'all' : 'custom'))

const conditionTypeOptions = computed(() => [
  { value: 'subscription', label: t('admin.announcements.form.conditionSubscription') },
  { value: 'balance', label: t('admin.announcements.form.conditionBalance') },
  { value: 'user', label: t('admin.announcements.form.conditionUser') }
])

const balanceOperatorOptions = computed(() => [
  { value: 'gt', label: t('admin.announcements.operators.gt') },
  { value: 'gte', label: t('admin.announcements.operators.gte') },
  { value: 'lt', label: t('admin.announcements.operators.lt') },
  { value: 'lte', label: t('admin.announcements.operators.lte') },
  { value: 'eq', label: t('admin.announcements.operators.eq') }
])

function setMode(next: Mode) {
  if (next === 'all') {
    emit('update:modelValue', { any_of: [] })
    return
  }
  if (anyOf.value.length === 0) {
    emit('update:modelValue', { any_of: [{ all_of: [defaultSubscriptionCondition()] }] })
  }
}

function defaultSubscriptionCondition(): AnnouncementCondition {
  return {
    type: 'subscription' as AnnouncementConditionType,
    operator: 'in' as AnnouncementOperator,
    group_ids: []
  }
}

function defaultBalanceCondition(): AnnouncementCondition {
  return {
    type: 'balance' as AnnouncementConditionType,
    operator: 'gte' as AnnouncementOperator,
    value: 0
  }
}

function defaultUserCondition(): AnnouncementCondition {
  return {
    type: 'user' as AnnouncementConditionType,
    operator: 'in' as AnnouncementOperator,
    user_ids: []
  }
}

type TargetingDraft = {
  any_of: AnnouncementConditionGroup[]
}

function updateTargeting(mutator: (draft: TargetingDraft) => void) {
  const draft: TargetingDraft = JSON.parse(JSON.stringify(props.modelValue ?? { any_of: [] }))
  if (!draft.any_of) draft.any_of = []
  mutator(draft)
  emit('update:modelValue', draft)
}

function addOrGroup() {
  updateTargeting((draft) => {
    if (draft.any_of.length >= 50) return
    draft.any_of.push({ all_of: [defaultSubscriptionCondition()] })
  })
}

function removeOrGroup(groupIndex: number) {
  updateTargeting((draft) => {
    draft.any_of.splice(groupIndex, 1)
  })
}

function addAndCondition(groupIndex: number) {
  updateTargeting((draft) => {
    const group = draft.any_of[groupIndex]
    if (!group.all_of) group.all_of = []
    if (group.all_of.length >= 50) return
    group.all_of.push(defaultSubscriptionCondition())
  })
}

function removeAndCondition(groupIndex: number, condIndex: number) {
  updateTargeting((draft) => {
    const group = draft.any_of[groupIndex]
    if (!group?.all_of) return
    group.all_of.splice(condIndex, 1)
  })
}

function setConditionType(groupIndex: number, condIndex: number, nextType: AnnouncementConditionType) {
  updateTargeting((draft) => {
    const group = draft.any_of[groupIndex]
    if (!group?.all_of) return

    if (nextType === 'subscription') {
      group.all_of[condIndex] = defaultSubscriptionCondition()
    } else if (nextType === 'user') {
      group.all_of[condIndex] = defaultUserCondition()
    } else {
      group.all_of[condIndex] = defaultBalanceCondition()
    }
  })
}

function setOperator(groupIndex: number, condIndex: number, op: AnnouncementOperator) {
  updateTargeting((draft) => {
    const group = draft.any_of[groupIndex]
    if (!group?.all_of) return

    const cond = group.all_of[condIndex]
    if (!cond) return

    cond.operator = op
  })
}

function setBalanceValue(groupIndex: number, condIndex: number, raw: string) {
  const n = raw === '' ? 0 : Number(raw)
  updateTargeting((draft) => {
    const group = draft.any_of[groupIndex]
    if (!group?.all_of) return

    const cond = group.all_of[condIndex]
    if (!cond) return

    cond.value = Number.isFinite(n) ? n : 0
  })
}

type UserTargetState = {
  query: string
  results: SimpleUser[]
  loading: boolean
  searched: boolean
}

const userTargetStates = reactive<Record<string, UserTargetState>>({})

function userStateKey(groupIndex: number, condIndex: number) {
  return `${groupIndex}:${condIndex}`
}

function ensureUserStatePath(groupIndex: number, condIndex: number) {
  const key = userStateKey(groupIndex, condIndex)
  if (!userTargetStates[key]) {
    userTargetStates[key] = { query: '', results: [], loading: false, searched: false }
  }
  return userTargetStates[key]
}

function userTargetState(groupIndex: number, condIndex: number) {
  return ensureUserStatePath(groupIndex, condIndex)
}

async function searchTargetUsers(groupIndex: number, condIndex: number) {
  const state = ensureUserStatePath(groupIndex, condIndex)
  const query = state.query.trim()
  if (query.length < 2) {
    state.results = []
    state.searched = true
    return
  }

  state.loading = true
  state.searched = false
  try {
    const results = await adminAPI.usage.searchUsers(query)
    state.results = results.filter((user) => !user.deleted)
  } catch (error) {
    console.error('Failed to search announcement users:', error)
    state.results = []
  } finally {
    state.loading = false
    state.searched = true
  }
}

function isUserSelected(cond: AnnouncementCondition, userId: number) {
  return (cond.user_ids ?? []).includes(userId)
}

function selectedUserLabel(groupIndex: number, condIndex: number, userId: number) {
  const user = userTargetState(groupIndex, condIndex).results.find((item) => item.id === userId)
  return user?.email ?? `#${userId}`
}

function addUserTarget(groupIndex: number, condIndex: number, userId: number) {
  updateTargeting((draft) => {
    const condition = draft.any_of[groupIndex]?.all_of?.[condIndex]
    if (!condition || condition.type !== 'user') return
    condition.user_ids = Array.from(new Set([...(condition.user_ids ?? []), userId]))
  })
}

function removeUserTarget(groupIndex: number, condIndex: number, userId: number) {
  updateTargeting((draft) => {
    const condition = draft.any_of[groupIndex]?.all_of?.[condIndex]
    if (!condition || condition.type !== 'user') return
    condition.user_ids = (condition.user_ids ?? []).filter((id) => id !== userId)
  })
}

// We keep group_ids selection in a parallel reactive map because GroupSelector is numeric list.
// Then we mirror it back to targeting.group_ids via a watcher.
const subscriptionSelections = reactive<Record<number, Record<number, number[]>>>({})

function ensureSelectionPath(groupIndex: number, condIndex: number) {
  if (!subscriptionSelections[groupIndex]) subscriptionSelections[groupIndex] = {}
  if (!subscriptionSelections[groupIndex][condIndex]) subscriptionSelections[groupIndex][condIndex] = []
}

// Sync from modelValue to subscriptionSelections (one-way: model -> local state)
watch(
  () => props.modelValue,
  (v) => {
    const groups = v?.any_of ?? []
    for (let gi = 0; gi < groups.length; gi++) {
      const allOf = groups[gi]?.all_of ?? []
      for (let ci = 0; ci < allOf.length; ci++) {
        const c = allOf[ci]
        if (c?.type === 'subscription') {
          ensureSelectionPath(gi, ci)
          // Only update if different to avoid triggering unnecessary updates
          const newIds = (c.group_ids ?? []).slice()
          const currentIds = subscriptionSelections[gi]?.[ci] ?? []
          if (JSON.stringify(newIds.sort()) !== JSON.stringify(currentIds.sort())) {
            subscriptionSelections[gi][ci] = newIds
          }
        }
      }
    }
  },
  { immediate: true }
)

// Sync from subscriptionSelections to modelValue (one-way: local state -> model)
// Use a debounced approach to avoid infinite loops
let syncTimeout: ReturnType<typeof setTimeout> | null = null
watch(
  () => subscriptionSelections,
  () => {
    // Debounce the sync to avoid rapid fire updates
    if (syncTimeout) clearTimeout(syncTimeout)

    syncTimeout = setTimeout(() => {
      // Build the new targeting state
      const newTargeting: TargetingDraft = JSON.parse(JSON.stringify(props.modelValue ?? { any_of: [] }))
      if (!newTargeting.any_of) newTargeting.any_of = []

      const groups = newTargeting.any_of ?? []
      for (let gi = 0; gi < groups.length; gi++) {
        const allOf = groups[gi]?.all_of ?? []
        for (let ci = 0; ci < allOf.length; ci++) {
          const c = allOf[ci]
          if (c?.type === 'subscription') {
            ensureSelectionPath(gi, ci)
            c.operator = 'in' as AnnouncementOperator
            c.group_ids = (subscriptionSelections[gi]?.[ci] ?? []).slice()
          }
        }
      }

      // Only emit if there's an actual change (deep comparison)
      if (JSON.stringify(props.modelValue) !== JSON.stringify(newTargeting)) {
        emit('update:modelValue', newTargeting)
      }
    }, 0)
  },
  { deep: true }
)

const validationError = computed(() => {
  if (mode.value !== 'custom') return ''

  const groups = anyOf.value
  if (groups.length === 0) return t('admin.announcements.form.addOrGroup')

  if (groups.length > 50) return 'any_of > 50'

  for (const g of groups) {
    const allOf = g?.all_of ?? []
    if (allOf.length === 0) return t('admin.announcements.form.addAndCondition')
    if (allOf.length > 50) return 'all_of > 50'

    for (const c of allOf) {
      if (c.type === 'subscription') {
        if (!c.group_ids || c.group_ids.length === 0) return t('admin.announcements.form.selectPackages')
      } else if (c.type === 'user') {
        if (!c.user_ids || c.user_ids.length === 0) return t('admin.announcements.form.selectUsers')
      }
    }
  }

  return ''
})
</script>
