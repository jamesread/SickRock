<template>
  <div v-if="visible" class="modal-overlay command-palette-overlay" @click="close" @keydown.escape="close" tabindex="0">
    <div class="modal-content command-palette" @click.stop>
      <div class="command-palette-header">
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          placeholder="Type a command or search..."
          class="command-palette-input"
          @keydown.down.prevent="selectNext"
          @keydown.up.prevent="selectPrevious"
          @keydown.enter.prevent="executeSelected"
          @keydown.escape="close"
        />
      </div>
      <div class="command-palette-results">
        <div
          v-for="(command, index) in filteredCommands"
          :key="command.id"
          :class="['command-item', { selected: index === selectedIndex }]"
          @click="executeCommand(command)"
          @mouseenter="selectedIndex = index"
        >
          <div class="command-icon">
            <HugeiconsIcon v-if="command.icon" :icon="command.icon" />
          </div>
          <div class="command-content">
            <div class="command-title">{{ command.title }}</div>
            <div v-if="command.description" class="command-description">{{ command.description }}</div>
          </div>
          <div v-if="command.shortcut" class="command-shortcut">
            <kbd v-for="(key, idx) in formatShortcut(command.shortcut)" :key="idx">{{ key }}</kbd>
          </div>
        </div>
        <div v-if="filteredCommands.length === 0" class="command-empty">
          No commands found
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { HomeIcon, DatabaseIcon, Settings01Icon, QuestionIcon } from '@hugeicons/core-free-icons'

export interface Command {
  id: string
  title: string
  description?: string
  icon?: any
  shortcut?: string
  action: () => void
  category?: string
}

const props = defineProps<{
  visible: boolean
  commands?: Command[]
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'command-executed': [command: Command]
  'open-shortcuts-help': []
}>()

const router = useRouter()
const searchInput = ref<HTMLInputElement | null>(null)
const searchQuery = ref('')
const selectedIndex = ref(0)

const isMac = computed(() => {
  return navigator.platform.toUpperCase().indexOf('MAC') >= 0
})

const defaultCommands: Command[] = [
  {
    id: 'keyboard-shortcuts',
    title: 'Keyboard Shortcuts',
    description: 'View all available keyboard shortcuts',
    icon: QuestionIcon,
    shortcut: 'g ?',
    action: () => {
      emit('open-shortcuts-help')
      close()
    },
    category: 'Help'
  },
  {
    id: 'home',
    title: 'Go to Home',
    description: 'Navigate to the home dashboard',
    icon: HomeIcon,
    shortcut: 'g h',
    action: () => router.push('/'),
    category: 'Navigation'
  },
  {
    id: 'tables',
    title: 'Go to Tables',
    description: 'View all tables',
    icon: DatabaseIcon,
    shortcut: 'g t',
    action: () => {
      // Navigate to first table or tables list if available
      router.push('/')
    },
    category: 'Navigation'
  },
  {
    id: 'control-panel',
    title: 'Go to Control Panel',
    description: 'Open the control panel',
    icon: Settings01Icon,
    shortcut: 'g c',
    action: () => router.push('/admin/control-panel'),
    category: 'Navigation'
  },
]

const allCommands = computed(() => {
  return [...defaultCommands, ...(props.commands || [])]
})

const filteredCommands = computed(() => {
  if (!searchQuery.value.trim()) {
    return allCommands.value
  }

  const query = searchQuery.value.toLowerCase()
  return allCommands.value.filter(cmd =>
    cmd.title.toLowerCase().includes(query) ||
    cmd.description?.toLowerCase().includes(query) ||
    cmd.category?.toLowerCase().includes(query)
  )
})

watch(() => props.visible, (newVal) => {
  if (newVal) {
    searchQuery.value = ''
    selectedIndex.value = 0
    nextTick(() => {
      searchInput.value?.focus()
    })
  }
})

watch(filteredCommands, () => {
  selectedIndex.value = 0
})

function selectNext() {
  if (selectedIndex.value < filteredCommands.value.length - 1) {
    selectedIndex.value++
  }
}

function selectPrevious() {
  if (selectedIndex.value > 0) {
    selectedIndex.value--
  }
}

function executeSelected() {
  if (filteredCommands.value[selectedIndex.value]) {
    executeCommand(filteredCommands.value[selectedIndex.value])
  }
}

function executeCommand(command: Command) {
  command.action()
  emit('command-executed', command)
  close()
}

function close() {
  emit('update:visible', false)
}

function formatShortcut(shortcut: string): string[] {
  return shortcut.split(' ').map(key => {
    if (key === 'g') return 'G'
    if (key === 'h') return 'H'
    if (key === 't') return 'T'
    if (key === 'c') return 'C'
    return key.toUpperCase()
  })
}
</script>

<style scoped>
.command-palette-overlay {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 10vh;
}

.command-palette {
  max-width: 600px;
  width: 90%;
  max-height: 70vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.command-palette-header {
  padding: 1rem;
  border-bottom: 1px solid #ddd;
}

.command-palette-input {
  width: 100%;
  padding: 0.75rem;
  font-size: 1rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  outline: none;
}

.command-palette-input:focus {
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.command-palette-results {
  flex: 1;
  overflow-y: auto;
  max-height: 60vh;
}

.command-item {
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background-color 0.15s;
}

.command-item:hover,
.command-item.selected {
  background-color: #f8f9fa;
}

.command-icon {
  width: 24px;
  height: 24px;
  margin-right: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666;
}

.command-content {
  flex: 1;
  min-width: 0;
}

.command-title {
  font-weight: 500;
  color: #333;
  margin-bottom: 0.25rem;
}

.command-description {
  font-size: 0.875rem;
  color: #666;
}

.command-shortcut {
  display: flex;
  gap: 0.25rem;
  margin-left: 1rem;
}

.command-shortcut kbd {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  background: white;
  border: 1px solid #ddd;
  border-radius: 3px;
  font-family: monospace;
  font-size: 0.75rem;
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
}

.command-empty {
  padding: 2rem;
  text-align: center;
  color: #999;
}
</style>
