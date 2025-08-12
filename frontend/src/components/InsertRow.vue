<script setup lang="ts">
import { ref } from 'vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const props = defineProps<{ tableId: string, fieldDefs: Array<{ name: string; type: string; required: boolean }> }>()
const emit = defineEmits<{ created: [] }>()

const form = ref<Record<string, any>>({})
const loading = ref(false)
const error = ref<string | null>(null)

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function submit() {
  const payload: Record<string, any> = {}
  for (const f of props.fieldDefs) {
    if (f.name === 'id') continue
    const v = form.value[f.name]
    if (v == null || v === '') continue
    payload[f.name] = f.type === 'int64' ? Number(v) : v
  }
  if (!payload.name && props.fieldDefs.find(d => d.name === 'name')) return
  loading.value = true
  error.value = null
  try {
    await client.createItem({ pageId: props.tableId, name: String(payload.name ?? '') })
    form.value = {}
    emit('created')
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <form @submit.prevent="submit">
      <template v-for="f in fieldDefs" :key="f.name">
        <template v-if="f.name !== 'id'">
            <label :for="'field-' + f.name">{{ f.name }}<span v-if="f.required" aria-label="required" title="required">*</span></label>
            <input
              v-if="f.type === 'string'"
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
            <input
              v-else-if="f.type === 'int64'"
              v-model.number="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="number"
              @keyup.enter="submit"
            />
        </template>
      </template>
      <button type="submit" :disabled="loading">Add</button>
    </form>
    <div v-if="error">{{ error }}</div>
  </div>
</template>



<style scoped>
form {
  grid-template-columns: max-content 1fr;
  gap: 1em;
}

</style>