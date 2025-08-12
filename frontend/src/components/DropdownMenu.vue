<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { MoreVerticalSquare01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/vue'

const dropdown = ref<HTMLDivElement | null>(null)
const open = ref(false)

let nextId = (window as any).__dropdownMenuNextId || 1
;(window as any).__dropdownMenuNextId = nextId + 1
const id = nextId

function positionMenu() {
  if (!dropdown.value) return
  if (dropdown.value.clientLeft + dropdown.value.clientWidth > window.innerWidth) {
    dropdown.value.style.left = '-50px'
  } else {
    dropdown.value.style.left = '25px'
  }
}

function broadcastOpen() {
  const evt = new CustomEvent('app-dropdown-open', { detail: id })
  window.dispatchEvent(evt)
}

function onAnyOpen(e: Event) {
  const detail = (e as CustomEvent).detail
  if (detail !== id && open.value) open.value = false
}

function toggle() {
  const willOpen = !open.value
  open.value = willOpen
  if (willOpen) {
    broadcastOpen()
    positionMenu()
  }
}

const items = ref<{ type: 'router-link' | 'callback'; path?: string; label: string; callback?: () => void }[]>([])

function addRouterLink(path: string, label: string) {
  items.value.push({ type: 'router-link', path, label })
}

function addCallback(label: string, callback: () => void, styleClass?: string) {
  items.value.push({ type: 'callback', label, callback, styleClass })
}

defineExpose({
  addRouterLink,
  addCallback,
})

onMounted(() => {
  window.addEventListener('app-dropdown-open', onAnyOpen as EventListener)
})

onBeforeUnmount(() => {
  window.removeEventListener('app-dropdown-open', onAnyOpen as EventListener)
})
</script>

<template>
  <div class="dropdown">
    <button type="button" class="button" @click="toggle">
      <HugeiconsIcon :icon="MoreVerticalSquare01Icon" width="16" height="16" />
    </button>
    <div v-if="open" class="dropdown-menu" ref = "dropdown">
      <ul>
        <li v-for="item in items" :key="item.label">
          <router-link v-if="item.type === 'router-link'" :to="item.path">{{ item.label }}</router-link>
          <a v-if="item.type === 'callback'" :class="item.styleClass" @click="item.callback">{{ item.label }}</a>
        </li>
      </ul>
    </div>
  </div>

</template>

<style scoped>
button {
  border: 0;
}

.dropdown-menu {
  position: absolute;
  z-index: 10;
  background: #fff;
  border: 1px solid #ddd;
  padding: 0;
  box-shadow: 0 2px 6px rgba(0, 0, 0, .08);
  min-width: 4em;
  min-height: 4rem;
}

ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

li {
  padding-left: 0;
  margin: 0;
  padding: 0;
}

li a {
  display: block;
  padding: .5rem;
  text-decoration: none;
  color: inherit;
}

li a:hover {
  background-color: #f0f0f0;
}

li label {
  border-radius: 0;
  display: block;
  margin: 0;
  padding: 0;
}
</style>
