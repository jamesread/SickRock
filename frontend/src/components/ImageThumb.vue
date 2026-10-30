<script setup lang="ts">
// Shows an image from the images system table. The file is fetched with the session token
// (see utils/images.ts), so it works whether or not the session cookie is set.
import { ref, watch } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ImageNotFound01Icon } from '@hugeicons/core-free-icons'
import { loadImageObjectUrl } from '../utils/images'

const props = withDefaults(defineProps<{
  imageId: string | number
  alt?: string
  /** thumb: small, for table cells; tile: fills a picker card; preview: form fields; full: item pages. */
  size?: 'thumb' | 'tile' | 'preview' | 'full'
}>(), {
  alt: '',
  size: 'preview',
})

const src = ref<string | null>(null)
const failed = ref(false)

watch(() => String(props.imageId), async (id) => {
  src.value = null
  failed.value = false
  try {
    const url = await loadImageObjectUrl(id)
    if (String(props.imageId) === id) src.value = url
  } catch {
    if (String(props.imageId) === id) failed.value = true
  }
}, { immediate: true })
</script>

<template>
  <span v-if="failed" class="image-thumb-missing subtle" title="Image unavailable">
    <HugeiconsIcon :icon="ImageNotFound01Icon" width="1.25em" height="1.25em" aria-hidden="true" />
    <span v-if="size !== 'thumb' && size !== 'tile'">Image unavailable</span>
  </span>
  <img
    v-else-if="src"
    :src="src"
    :alt="alt"
    :class="['image-thumb', `image-thumb-${size}`]"
    loading="lazy"
    decoding="async"
  />
  <span v-else :class="['image-thumb-loading', `image-thumb-${size}`]" aria-busy="true" />
</template>

<style scoped>
.image-thumb {
  display: block;
  object-fit: cover;
  border-radius: 0.25rem;
}

.image-thumb-thumb {
  width: 2.5rem;
  height: 2.5rem;
}

.image-thumb-tile {
  width: 100%;
  aspect-ratio: 4 / 3;
}

.image-thumb-preview {
  max-width: min(16rem, 100%);
  max-height: 12rem;
  object-fit: contain;
}

.image-thumb-full {
  max-width: 100%;
  max-height: 32rem;
  object-fit: contain;
}

.image-thumb-loading.image-thumb-thumb {
  display: inline-block;
}

.image-thumb-loading.image-thumb-tile {
  display: block;
}

.image-thumb-loading.image-thumb-preview,
.image-thumb-loading.image-thumb-full {
  display: block;
  width: 8rem;
  height: 6rem;
}

.image-thumb-missing {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
</style>
