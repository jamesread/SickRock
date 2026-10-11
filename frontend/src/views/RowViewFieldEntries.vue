<script setup lang="ts">
import { inject } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Edit03Icon, FileEditIcon, SquareLock01Icon } from '@hugeicons/core-free-icons'
import UserRefChip from '../components/UserRefChip.vue'
import { rowViewFieldUiKey, type RowViewFieldUi } from './rowViewFieldUi'

export type FieldEntry = {
  key: string
  displayValue: unknown
  valueClass: string
}

defineProps<{
  entries: FieldEntry[]
}>()

const ui = inject(rowViewFieldUiKey) as RowViewFieldUi
const { editingField, editingValue, savingField, users } = ui
</script>

<template>
  <dl class="row-fields">
    <template v-for="entry in entries" :key="entry.key">
      <dt>{{ ui.displayFieldLabel(entry.key) }}</dt>
      <dd :class="entry.valueClass">
        <div
          v-if="editingField === entry.key"
          class="inline-edit"
          @click.stop
        >
          <input
            v-if="ui.isTinyintColumn(entry.key)"
            type="checkbox"
            :checked="editingValue === '1'"
            :disabled="savingField"
            class="edit-checkbox"
            @change="event => {
              editingValue = (event.target as HTMLInputElement).checked ? '1' : '0'
              ui.saveField()
            }"
          />
          <select
            v-else-if="ui.isUserRefColumn(entry.key)"
            :ref="ui.setEditInput"
            v-model="editingValue"
            :disabled="savingField"
            class="edit-input"
            @change="ui.saveField"
            @keyup.escape="ui.cancelEdit"
          >
            <option v-if="!ui.getFieldDef(entry.key)?.required" value="">— None —</option>
            <option
              v-for="user in users"
              :key="user.id"
              :value="String(user.id)"
            >
              {{ user.username || `User #${user.id}` }}
            </option>
          </select>
          <input
            v-else-if="ui.isDatetimeColumn(entry.key)"
            :ref="ui.setEditInput"
            v-model="editingValue"
            type="datetime-local"
            :disabled="savingField"
            class="edit-input"
            @keyup.enter="ui.saveField"
            @keyup.escape="ui.cancelEdit"
            @blur="ui.saveField"
          />
          <input
            v-else
            :ref="ui.setEditInput"
            v-model="editingValue"
            type="text"
            :disabled="savingField"
            class="edit-input"
            @keyup.enter="ui.saveField"
            @keyup.escape="ui.cancelEdit"
            @blur="ui.saveField"
          />
        </div>
        <div
          v-else
          class="field-display"
          :class="{
            editable: ui.canEditField(entry.key, entry.valueClass),
            'has-full-edit': ui.showFullEditIcon(entry.key, entry.valueClass),
            'has-read-only': ui.showReadOnlyIcon(entry.key, entry.valueClass),
          }"
          @click="ui.startEdit(entry.key, entry.valueClass)"
        >
          <template v-if="ui.isUserRefColumn(entry.key) && ui.hasStoredUserRefValue(ui.resolveRawFieldValue(entry.key))">
            <UserRefChip
              :label="ui.displayUserLabel(ui.resolveRawFieldValue(entry.key))"
              :to="ui.canEditField(entry.key, entry.valueClass)
                ? undefined
                : `/admin/iam/users/${ui.resolveRawFieldValue(entry.key)}`"
            />
          </template>
          <template v-else-if="ui.getFkDisplay(entry.key).to">
            <router-link :to="ui.getFkDisplay(entry.key).to">{{ ui.getFkDisplay(entry.key).label }}</router-link>
          </template>
          <template v-else-if="entry.valueClass === 'markdown-content'">
            <div v-html="entry.displayValue"></div>
          </template>
          <template v-else-if="ui.isTinyintColumn(entry.key)">
            <span :class="ui.getBooleanValue(entry.key) ? 'boolean-true' : 'boolean-false'">
              {{ ui.getBooleanValue(entry.key) ? 'Yes' : 'No' }}
            </span>
          </template>
          <template v-else>
            {{ entry.displayValue }}
          </template>
          <HugeiconsIcon
            v-if="ui.canEditField(entry.key, entry.valueClass)"
            :icon="Edit03Icon"
            class="inline-edit-icon"
            width="0.9em"
            height="0.9em"
            aria-hidden="true"
          />
          <router-link
            v-else-if="ui.showFullEditIcon(entry.key, entry.valueClass)"
            :to="ui.fullEditTo()"
            class="full-edit-icon"
            :title="ui.fullEditLabel()"
            :aria-label="ui.fullEditLabel()"
            @click.stop
          >
            <HugeiconsIcon
              :icon="FileEditIcon"
              width="0.9em"
              height="0.9em"
              aria-hidden="true"
            />
          </router-link>
          <span
            v-else-if="ui.showReadOnlyIcon(entry.key, entry.valueClass)"
            class="read-only-icon"
            title="Read only"
            aria-label="Read only"
          >
            <HugeiconsIcon
              :icon="SquareLock01Icon"
              width="0.9em"
              height="0.9em"
              aria-hidden="true"
            />
          </span>
        </div>
      </dd>
    </template>
  </dl>
</template>

<style scoped>
.row-fields {
  display: grid;
  grid-template-columns: 12rem minmax(0, 1fr);
  column-gap: 1rem;
  margin: 0;
}

.row-fields dt {
  width: 12rem;
  margin: 0;
  font-weight: 600;
}

.row-fields dd {
  margin: 0;
  min-width: 0;
}

.field-display.editable,
.field-display.has-full-edit,
.field-display.has-read-only {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.field-display.editable {
  cursor: pointer;
}

.inline-edit-icon,
.full-edit-icon,
.read-only-icon {
  flex-shrink: 0;
  color: var(--muted-text-color, #8a8a8a);
  opacity: 0.55;
}

.read-only-icon {
  display: inline-flex;
  line-height: 0;
}

.full-edit-icon {
  display: inline-flex;
  line-height: 0;
  text-decoration: none;
}

.field-display.editable:hover .inline-edit-icon,
.full-edit-icon:hover {
  opacity: 0.9;
}
</style>
