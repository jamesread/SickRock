<template>
  <div class="device-code-login">
    <p style="font-size: 0.875rem; color: #6b7280; margin-bottom: 1rem;">
      Generate a device code to authenticate on another device. Enter the code on the other device to complete login.
    </p>

    <div v-if="!deviceCode" class="generate-section">
      <button
        type="button"
        @click="handleGenerateCode"
        :disabled="isGenerating"
        style="width: 100%; padding: 0.75rem 1rem; border-radius: 0.375rem; background-color: #2563eb; color: white; border: none; cursor: pointer; font-size: 0.875rem; font-weight: 500;"
        :style="{ backgroundColor: isGenerating ? '#9ca3af' : '#2563eb', cursor: isGenerating ? 'not-allowed' : 'pointer' }"
      >
        <span v-if="isGenerating">Generating...</span>
        <span v-else>Generate Device Code</span>
      </button>
    </div>

    <div v-else class="code-display-section">
      <div style="text-align: center; margin-bottom: 1rem;">
        <p style="font-size: 0.875rem; color: #374151; margin-bottom: 0.5rem; font-weight: 500;">Your device code:</p>
        <div
          style="
            display: inline-block;
            padding: 1rem 2rem;
            background-color: #f3f4f6;
            border: 2px solid #d1d5db;
            border-radius: 0.5rem;
            font-family: monospace;
            font-size: 2rem;
            letter-spacing: 0.2em;
            font-weight: 600;
            color: #111827;
            margin-bottom: 0.5rem;
          "
        >
          {{ deviceCode }}
        </div>
        <p v-if="deviceCodeExpiresAt > 0" style="font-size: 0.75rem; color: #6b7280; margin-top: 0.5rem;">
          Expires: {{ formatTime(deviceCodeExpiresAt) }}
        </p>
      </div>

      <div
        style="
          padding: 0.75rem 1rem;
          background-color: #eff6ff;
          border: 1px solid #bfdbfe;
          border-radius: 0.375rem;
          font-size: 0.875rem;
          color: #1e40af;
          margin-bottom: 1rem;
        "
      >
        <p style="margin: 0;">
          Enter this code on the other device to complete authentication. This page will automatically redirect when the code is claimed.
        </p>
      </div>

      <button
        type="button"
        @click="handleCancel"
        style="width: 100%; padding: 0.5rem 1rem; border-radius: 0.375rem; background-color: #f3f4f6; color: #374151; border: 1px solid #d1d5db; cursor: pointer; font-size: 0.875rem;"
      >
        Cancel
      </button>
    </div>

    <div v-if="error" role="alert" style="margin-top: 1rem;">
      <div
        style="
          padding: 0.75rem 1rem;
          background-color: #fef2f2;
          border: 1px solid #fecaca;
          border-radius: 0.375rem;
          font-size: 0.875rem;
          color: #991b1b;
        "
      >
        {{ error }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { useDeviceCodeLogin } from '../composables/useDeviceCodeLogin'

const { deviceCode, deviceCodeExpiresAt, generateDeviceCode, stopDeviceCodePolling, formatTime } = useDeviceCodeLogin()

const isGenerating = ref(false)
const error = ref('')

const handleGenerateCode = async () => {
  isGenerating.value = true
  error.value = ''

  try {
    await generateDeviceCode()
  } catch (err) {
    console.error('Failed to generate device code:', err)
    error.value = 'Failed to generate device code. Please try again.'
  } finally {
    isGenerating.value = false
  }
}

const handleCancel = () => {
  stopDeviceCodePolling()
  deviceCode.value = ''
  deviceCodeExpiresAt.value = 0
  error.value = ''
}

// Clean up polling when component is unmounted
onUnmounted(() => {
  stopDeviceCodePolling()
})
</script>

<style scoped>
.device-code-login {
  padding: 1rem 0;
}

.generate-section,
.code-display-section {
  margin-bottom: 1rem;
}
</style>
