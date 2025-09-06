<script setup lang="ts">
import { ref } from 'vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const props = defineProps<{ tableId: string }>()
const emit = defineEmits<{ added: [] }>()

const name = ref('')
const type = ref<'string' | 'int64' | 'tinyint' | 'datetime'>('string')
const required = ref(false)
const defaultToCurrentTimestamp = ref(false)
const loading = ref(false)
const error = ref<string | null>(null)

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function submit() {
    if (!name.value || loading.value) return
    loading.value = true
    error.value = null
    try {
        await client.addTableColumn({
            pageId: props.tableId,
            field: {
                name: name.value,
                type: type.value,
                required: required.value,
                defaultToCurrentTimestamp: defaultToCurrentTimestamp.value
            }
        })
        name.value = ''
        required.value = false
        defaultToCurrentTimestamp.value = false
        type.value = 'string'
        emit('added')
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
            <label for="addcol-name">Column Name</label>
            <div class="flex-row">
                <input id="addcol-name" v-model="name" placeholder="Column name" type="text" />
                <div class="desc">This will be the identifier for the column in the table.</div>
            </div>

            <label for="addcol-type">Column Type</label>
            <div class="flex-row">
                <select id="addcol-type" v-model="type">
                    <option value="string">string</option>
                    <option value="int64">int64</option>
                    <option value="tinyint">tinyint</option>
                    <option value="datetime">datetime</option>
                </select>
            </div>

            <label for="addcol-required">Required</label>
            <div class="flex-row">
                <input id="addcol-required" type="checkbox" v-model="required" />
                <div class="desc">If checked, this column cannot be left empty.</div>
            </div>

            <template v-if="type === 'datetime'">
                <label for="addcol-default-timestamp">Default to Current Timestamp</label>
                <div class="flex-row">
                    <input id="addcol-default-timestamp" type="checkbox" v-model="defaultToCurrentTimestamp" />
                    <div class="desc">If checked, new rows will automatically get the current timestamp for this column.</div>
                </div>
            </template>

            <button id="addcol-submit" type="submit" :disabled="loading || !name">Add column</button>
        </form>
        <div v-if="error">{{ error }}</div>
    </div>
</template>
