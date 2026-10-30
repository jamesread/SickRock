<script setup lang="ts">
// Edits an image column: upload a file, take a photo with the browser camera, or paste an image
// from the clipboard (the Paste button, or Ctrl+V while the field has focus). The image is
// uploaded straight away (UploadImage) and the field's value becomes the new images.id; the row
// itself is saved by the surrounding form.
import { computed, nextTick, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Camera01Icon, CameraRotated01Icon, ClipboardPasteIcon, Delete02Icon, Upload04Icon } from '@hugeicons/core-free-icons'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import ImageThumb from './ImageThumb.vue'
import { createApiClient } from '../stores/api'
import { useCameraCapture } from '../composables/useCameraCapture'
import { hasImageValue } from '../utils/fieldTypes'
import {
  ACCEPTED_IMAGE_TYPES,
  MAX_IMAGE_BYTES,
  formatBytes,
  pastedImageFilename,
  pickClipboardImageType,
} from '../utils/images'

const props = withDefaults(defineProps<{
  modelValue?: string | null
  /** Table configuration the column belongs to. */
  tableId: string
  columnName: string
  id?: string
  disabled?: boolean
  clearable?: boolean
}>(), {
  modelValue: '',
  clearable: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const client = createApiClient()
const camera = useCameraCapture()

const fileInput = ref<HTMLInputElement | null>(null)
const dialogEl = ref<HTMLDialogElement | null>(null)
const video = ref<HTMLVideoElement | null>(null)
const uploading = ref(false)
const error = ref<string | null>(null)

/** navigator.clipboard.read needs a secure context (HTTPS or localhost). */
const canReadClipboard = typeof navigator !== 'undefined' && typeof navigator.clipboard?.read === 'function'

const hasImage = computed(() => hasImageValue(props.modelValue))
const busy = computed(() => props.disabled || uploading.value)

async function upload(blob: Blob, filename: string) {
  if (blob.size > MAX_IMAGE_BYTES) {
    error.value = `That image is ${formatBytes(blob.size)}; the largest allowed is ${formatBytes(MAX_IMAGE_BYTES)}.`
    return
  }
  uploading.value = true
  error.value = null
  try {
    const res = await client.uploadImage({
      pageId: props.tableId,
      columnName: props.columnName,
      data: new Uint8Array(await blob.arrayBuffer()),
      filename,
    })
    if (res.image) {
      emit('update:modelValue', String(res.image.id))
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    uploading.value = false
  }
}

function chooseFile() {
  fileInput.value?.click()
}

async function onFileChosen(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) {
    await upload(file, file.name)
  }
}

async function openCamera() {
  error.value = null
  dialogEl.value?.showModal()
  await nextTick()
  if (video.value) {
    await camera.start(video.value)
  }
}

async function flipCamera() {
  if (video.value) {
    await camera.flip(video.value)
  }
}

async function takePhoto() {
  if (!video.value) return
  try {
    const photo = await camera.capture(video.value)
    dialogEl.value?.close()
    await upload(photo, `photo-${new Date().toISOString().replace(/[:.]/g, '-')}.jpg`)
  } catch (e) {
    camera.error.value = e instanceof Error ? e.message : String(e)
  }
}

async function pasteFromClipboard() {
  error.value = null
  try {
    for (const item of await navigator.clipboard.read()) {
      const type = pickClipboardImageType(item.types)
      if (type) {
        await upload(await item.getType(type), pastedImageFilename(type))
        return
      }
    }
    error.value = 'There is no image on the clipboard. Copy an image (PNG, JPEG, WebP or GIF) first.'
  } catch (e) {
    error.value = e instanceof DOMException && e.name === 'NotAllowedError'
      ? 'The browser did not allow reading the clipboard. Allow clipboard access, or press Ctrl+V here instead.'
      : (e instanceof Error ? e.message : String(e))
  }
}

/** Ctrl+V / Cmd+V while focus is inside the field. Text pastes into inputs are left alone. */
async function onPaste(event: ClipboardEvent) {
  if (busy.value) return
  const file = Array.from(event.clipboardData?.files ?? []).find(f => ACCEPTED_IMAGE_TYPES.includes(f.type))
  if (!file) return
  event.preventDefault()
  await upload(file, file.name && file.name !== 'image.png' ? file.name : pastedImageFilename(file.type))
}

function clear() {
  emit('update:modelValue', '')
}
</script>

<template>
  <div class="image-field" @paste="onPaste">
    <ImageThumb v-if="hasImage" :image-id="String(modelValue)" size="preview" />
    <span v-else class="subtle">No image</span>

    <div class="image-field-actions">
      <button
        v-if="camera.supported"
        :id="id"
        type="button"
        class="button inline-icon neutral"
        :disabled="busy"
        @click="openCamera"
      >
        <HugeiconsIcon :icon="Camera01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Take photo</span>
      </button>
      <button
        :id="camera.supported ? undefined : id"
        type="button"
        class="button inline-icon neutral"
        :disabled="busy"
        @click="chooseFile"
      >
        <HugeiconsIcon :icon="Upload04Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ hasImage ? 'Replace…' : 'Upload…' }}</span>
      </button>
      <button
        v-if="canReadClipboard"
        type="button"
        class="button inline-icon neutral"
        :disabled="busy"
        title="Paste an image from the clipboard"
        @click="pasteFromClipboard"
      >
        <HugeiconsIcon :icon="ClipboardPasteIcon" width="1em" height="1em" aria-hidden="true" />
        <span>Paste</span>
      </button>
      <button
        v-if="clearable && hasImage"
        type="button"
        class="button inline-icon neutral"
        :disabled="busy"
        @click="clear"
      >
        <HugeiconsIcon :icon="Delete02Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Remove</span>
      </button>
      <span v-if="uploading" class="subtle">Uploading…</span>
    </div>

    <input
      ref="fileInput"
      type="file"
      :accept="ACCEPTED_IMAGE_TYPES.join(',')"
      hidden
      @change="onFileChosen"
    />
    <NotificationBlock v-if="error" type="error" :message="error" />

    <dialog ref="dialogEl" class="dialog image-camera-dialog" @close="camera.stop()">
      <h2>Take a photo</h2>
      <video ref="video" muted playsinline autoplay />
      <NotificationBlock v-if="camera.error.value" type="warning" :message="camera.error.value" />
      <p v-else-if="camera.starting.value" class="subtle">Starting camera…</p>
      <div class="image-field-actions">
        <button
          type="button"
          class="button inline-icon good"
          :disabled="!camera.active.value"
          @click="takePhoto"
        >
          <HugeiconsIcon :icon="Camera01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Take photo</span>
        </button>
        <button
          type="button"
          class="button inline-icon neutral"
          :disabled="camera.starting.value"
          title="Switch between the front and rear cameras"
          @click="flipCamera"
        >
          <HugeiconsIcon :icon="CameraRotated01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Switch camera</span>
        </button>
        <button type="button" class="button neutral" @click="dialogEl?.close()">Cancel</button>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.image-field {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.5rem;
}

.image-field-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.image-camera-dialog video {
  display: block;
  width: 100%;
  max-height: 60vh;
  background: #000;
  border-radius: 0.25rem;
  margin-block: 0.5rem;
}
</style>
