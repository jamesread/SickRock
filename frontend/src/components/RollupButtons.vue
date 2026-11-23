<script setup lang="ts">
// A toolbar button per rollup that starts from this table (system table table_rollups). Running
// one replaces every row of its target table with grouped and summed detail rows, so it asks first.
// With a week (weekly plan views) only the source rows dated in that week are used.
import { computed, ref, watch } from 'vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { GitMergeIcon } from '@hugeicons/core-free-icons'
import { createApiClient } from '../stores/api'
import { useAuthStore } from '../stores/auth'
import type { Rollup, RunRollupResponse } from '../gen/sickrock_pb'
import type { DialogApi } from '../utils/dialog'
import { allRowsRollupRequest, rollupResultText, weekRollupRequest } from '../utils/rollups'

const props = defineProps<{
  tableId: string
  /** Source column dated by the week; with weekStart, runs cover only that week. */
  dateColumn?: string
  weekStart?: Date
  /** Describes the rows a run covers, e.g. "23 Nov – 29 Nov". */
  scopeLabel?: string
  /** Only these source rows (e.g. those a table's filters leave); undefined for every row. */
  sourceIds?: string[]
}>()

const client = createApiClient()
const auth = useAuthStore()
const rollups = ref<Rollup[]>([])
const selected = ref<Rollup | null>(null)
const running = ref(false)
const errorMessage = ref('')
const result = ref<RunRollupResponse | null>(null)
const dialogEl = ref<DialogApi | null>(null)

const canRun = computed(() => auth.hasPermission('rollup.run'))
const scope = computed(() => {
  if (props.scopeLabel) return `the rows of ${props.scopeLabel}`
  if (props.sourceIds) {
    const n = props.sourceIds.length
    return `the ${n} ${n === 1 ? 'row' : 'rows'} the current filters show`
  }
  return 'every row of this table'
})

async function load() {
  rollups.value = []
  if (!canRun.value || !props.tableId) return
  try {
    const res = await client.listRollups({ sourceTable: props.tableId })
    rollups.value = res.rollups
  } catch {
    // Rollups are optional; a failure just leaves no buttons.
  }
}

function open(rollup: Rollup) {
  selected.value = rollup
  errorMessage.value = ''
  result.value = null
  dialogEl.value?.show()
}

async function run() {
  const rollup = selected.value
  if (!rollup || running.value) return
  running.value = true
  errorMessage.value = ''
  try {
    const request = props.dateColumn && props.weekStart
      ? weekRollupRequest(rollup.id, props.dateColumn, props.weekStart)
      : allRowsRollupRequest(rollup.id, props.sourceIds)
    result.value = await client.runRollup(request)
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Could not run the rollup.'
  } finally {
    running.value = false
  }
}

watch(() => props.tableId, load, { immediate: true })
</script>

<template>
  <div class="rollup-buttons">
    <button
      v-for="rollup in rollups"
      :key="String(rollup.id)"
      type="button"
      class="button inline-icon neutral"
      :title="`Replace ${rollup.targetTable} with totals from ${scope}`"
      @click="open(rollup)"
    >
      <HugeiconsIcon :icon="GitMergeIcon" width="1em" height="1em" aria-hidden="true" />
      <span>{{ rollup.name }}…</span>
    </button>
    <Teleport to="body">
      <Dialog ref="dialogEl" :title="selected?.name ?? 'Rollup'">
        <template v-if="result">
          <p>{{ rollupResultText(result) }}</p>
          <NotificationBlock
            v-if="result.skipped.length > 0"
            type="warning"
            :message="`${result.skipped.length} values were not numbers and were left out.`"
          />
          <ul v-if="result.skipped.length > 0" class="subtle">
            <li v-for="line in result.skipped.slice(0, 10)" :key="line">{{ line }}</li>
          </ul>
        </template>
        <template v-else-if="selected">
          <p>
            Replaces everything in <strong>{{ selected.targetTable }}</strong> with totals from the
            {{ selected.detailTable }} of {{ scope }}.
          </p>
          <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
        </template>

        <template #actions>
          <router-link
            v-if="result"
            :to="`/table/${result.targetTable}`"
            class="button good"
            @click="dialogEl?.close()"
          >Open {{ result.targetTable }}</router-link>
          <button v-else type="button" class="good" :disabled="running" @click="run">
            {{ running ? 'Running…' : 'Run' }}
          </button>
        </template>
      </Dialog>
    </Teleport>
  </div>
</template>

<style scoped>
/* Unwrapped so the Section toolbar sees each button (and can fold them into its menu). */
.rollup-buttons {
  display: contents;
}
</style>
