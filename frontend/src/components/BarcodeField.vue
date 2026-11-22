<script setup lang="ts">
// Edits a barcode column: type the code, read it with a keyboard-wedge barcode scanner (which types
// it), or scan it with the device camera (BarcodeDetector, else the zxing-wasm ponyfill). A camera
// scan fills the field and emits `scanned` so callers can save straight away.
import { nextTick, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { BarCode01Icon } from '@hugeicons/core-free-icons'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import type { DialogApi } from '../utils/dialog'
import { useBarcodeCamera } from '../composables/useBarcodeCamera'

withDefaults(defineProps<{
  modelValue?: string | null
  id?: string
  placeholder?: string
  disabled?: boolean
}>(), {
  modelValue: '',
  placeholder: 'Type or scan a barcode',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  scanned: [value: string]
  enter: []
  escape: []
}>()

const dialogEl = ref<DialogApi | null>(null)
const video = ref<HTMLVideoElement | null>(null)

const camera = useBarcodeCamera((code) => {
  camera.stop()
  dialogEl.value?.close()
  emit('update:modelValue', code)
  emit('scanned', code)
})

async function openCamera() {
  dialogEl.value?.show()
  await nextTick()
  if (video.value) {
    await camera.start(video.value)
  }
}
</script>

<template>
  <div class="barcode-field">
    <input
      :id="id"
      :value="modelValue ?? ''"
      :placeholder="placeholder"
      :disabled="disabled"
      type="text"
      inputmode="text"
      autocomplete="off"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      @keyup.enter="emit('enter')"
      @keyup.escape="emit('escape')"
    />
    <button
      v-if="camera.supported"
      type="button"
      class="button inline-icon neutral"
      :disabled="disabled"
      @click="openCamera"
    >
      <HugeiconsIcon :icon="BarCode01Icon" width="1em" height="1em" aria-hidden="true" />
      <span>Scan</span>
    </button>

    <Dialog ref="dialogEl" title="Scan a barcode" class="barcode-camera-dialog" @close="camera.stop()">
      <video ref="video" muted playsinline autoplay />
      <NotificationBlock v-if="camera.error.value" type="warning" :message="camera.error.value" />
      <p v-else-if="camera.starting.value" class="subtle">Starting camera…</p>
      <p v-else class="subtle">Hold the barcode in front of the camera.</p>
    </Dialog>
  </div>
</template>

<style scoped>
.barcode-field {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

/* Wraps the Scan button onto its own line when there is no room for a usable input beside it. */
.barcode-field input {
  flex: 1 1 12rem;
  min-width: 0;
}

.barcode-camera-dialog video {
  display: block;
  width: 100%;
  max-height: 60vh;
  max-height: 60dvh;
  background: #000;
}
</style>
