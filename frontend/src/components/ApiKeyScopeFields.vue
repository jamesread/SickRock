<script setup lang="ts">
import { computed } from 'vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import {
  API_KEY_RESOURCE_TYPES,
  permissionOptions,
  resourceOptions,
  scopeWarnings,
  selectedResources,
  withResources,
  type ApiKeyResourceOption,
  type ApiKeyScope,
} from '../utils/apiKeyScope'

const props = withDefaults(defineProps<{
  availablePermissions: string[]
  availableResources: ApiKeyResourceOption[]
  disabled?: boolean
}>(), { disabled: false })

const scope = defineModel<ApiKeyScope>({ required: true })

function update(patch: Partial<ApiKeyScope>) {
  scope.value = { ...scope.value, ...patch }
}

const permissionChoices = computed(() => permissionOptions(props.availablePermissions, scope.value))

const resourceGroups = computed(() => API_KEY_RESOURCE_TYPES.map(({ type, plural }) => ({
  type,
  plural,
  options: resourceOptions(type, props.availableResources, scope.value),
  selected: selectedResources(type, scope.value),
})))

const warnings = computed(() => scopeWarnings(scope.value))
</script>

<template>
  <FormField
    label="Permissions"
    component-has-label
    description="A key never has more access than you do. If your access shrinks, so does the key's."
  >
    <RadioGroup
      :model-value="scope.restrictPermissions"
      name="api-key-permission-mode"
      :options="[
        { value: false, label: 'Everything I can do' },
        { value: true, label: 'Only the permissions I choose' },
      ]"
      :disabled="disabled"
      @update:model-value="update({ restrictPermissions: $event })"
    />
  </FormField>
  <FormField v-if="scope.restrictPermissions" label="Allowed permissions" component-has-label>
    <CheckGroup
      :model-value="scope.permissions"
      class="scope-checks"
      name="api-key-permissions"
      :options="permissionChoices"
      :disabled="disabled"
      @update:model-value="update({ permissions: $event })"
    />
  </FormField>

  <FormField
    label="Tables, dashboards and workflows"
    component-has-label
    description="System tables are governed by permissions instead."
  >
    <RadioGroup
      :model-value="scope.restrictResources"
      name="api-key-resource-mode"
      :options="[
        { value: false, label: 'Everything I can use' },
        { value: true, label: 'Only the ones I choose' },
      ]"
      :disabled="disabled"
      @update:model-value="update({ restrictResources: $event })"
    />
  </FormField>
  <template v-if="scope.restrictResources">
    <FormField
      v-for="group in resourceGroups"
      :key="group.type"
      :label="group.plural"
      component-has-label
    >
      <CheckGroup
        v-if="group.options.length"
        :model-value="group.selected"
        class="scope-checks"
        :name="`api-key-${group.type}s`"
        :options="group.options"
        :disabled="disabled"
        @update:model-value="scope = withResources(scope, group.type, $event)"
      />
      <p v-else class="subtle">None to choose from.</p>
    </FormField>
  </template>

  <NotificationBlock v-for="warning in warnings" :key="warning" type="warning" :message="warning" />
</template>

<style scoped>
/* Let long labels wrap inside the form column instead of overflowing it. */
.scope-checks {
  box-sizing: border-box;
  width: 100%;
}
</style>
