<script setup lang="ts">
// A right-click menu at the pointer, styled by picocrank's nav.menu.context-menu.
// Call open(event) from a contextmenu handler; put <li><button role="menuitem"> items in the
// default slot. It closes on a click anywhere (including an item), Escape, scrolling or resizing.
import { nextTick, onBeforeUnmount, ref } from 'vue'

defineProps<{
  label: string
}>()

const MARGIN_PX = 8

const isOpen = ref(false)
const menuEl = ref<HTMLElement | null>(null)
const position = ref({ top: '0px', left: '0px' })

async function open(event: MouseEvent) {
  event.preventDefault()
  event.stopPropagation()
  position.value = { top: `${event.clientY}px`, left: `${event.clientX}px` }
  isOpen.value = true
  await nextTick()
  keepOnScreen(event.clientX, event.clientY)
  menuEl.value?.querySelector<HTMLElement>('[role="menuitem"]')?.focus({ preventScroll: true })
  document.addEventListener('pointerdown', onPointerDown, true)
  document.addEventListener('keydown', onKeyDown)
  window.addEventListener('scroll', close, true)
  window.addEventListener('resize', close)
}

function close() {
  if (!isOpen.value) return
  isOpen.value = false
  document.removeEventListener('pointerdown', onPointerDown, true)
  document.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('scroll', close, true)
  window.removeEventListener('resize', close)
}

function keepOnScreen(x: number, y: number) {
  const rect = menuEl.value?.getBoundingClientRect()
  if (!rect) return
  const left = Math.max(MARGIN_PX, Math.min(x, window.innerWidth - rect.width - MARGIN_PX))
  const top = Math.max(MARGIN_PX, Math.min(y, window.innerHeight - rect.height - MARGIN_PX))
  position.value = { top: `${top}px`, left: `${left}px` }
}

function onPointerDown(event: PointerEvent) {
  if (!menuEl.value?.contains(event.target as Node)) close()
}

function onKeyDown(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

onBeforeUnmount(close)

defineExpose({ open, close })
</script>

<template>
  <Teleport to="body">
    <nav
      v-if="isOpen"
      ref="menuEl"
      class="menu context-menu"
      role="menu"
      :aria-label="label"
      :style="position"
      @click="close"
      @contextmenu.prevent
    >
      <ul class="noListStyle">
        <li>
          <div>
            <ul>
              <slot />
            </ul>
          </div>
        </li>
      </ul>
    </nav>
  </Teleport>
</template>
