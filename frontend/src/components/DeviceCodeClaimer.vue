<template>
  <Section title = "Claim Device Code" class = "small" style = "margin: auto">
    <div style="max-width: 28rem; margin: 0 auto;">
      <p style="font-size: 0.875rem; color: #6b7280; margin-bottom: 1rem;">Enter the 4-digit code shown on the other device to complete authentication.</p>

      <form @submit.prevent="claimDeviceCode" style="display: flex; gap: 0.75rem; align-items: flex-start; margin-bottom: 1rem;">
        <label for="device-code" class="sr-only">Device code</label>
        <input
          id="device-code"
          v-model="deviceCode"
          type="text"
          maxlength="4"
          inputmode="numeric"
          autocomplete="one-time-code"
          pattern="[0-9]{4}"
          placeholder="1234"
          :aria-invalid="!isValidCode && deviceCode.length > 0 ? 'true' : 'false'"
          style="flex: 1; padding: 0.5rem 1rem; border: 1px solid #d1d5db; border-radius: 0.375rem; text-align: center; letter-spacing: 0.1em; font-family: monospace; font-size: 1.25rem; outline: none;"
          :disabled="isLoading"
          @focus="($event.target as HTMLInputElement).style.borderColor = '#3b82f6'; ($event.target as HTMLInputElement).style.boxShadow = '0 0 0 2px rgba(59, 130, 246, 0.1)'"
          @blur="($event.target as HTMLInputElement).style.borderColor = '#d1d5db'; ($event.target as HTMLInputElement).style.boxShadow = 'none'"
        />

        <button
          type="submit"
          :disabled="isLoading || !isValidCode"
          style="padding: 0.5rem 1rem; border-radius: 0.375rem; background-color: #2563eb; color: white; border: none; cursor: pointer;"
          :style="{ backgroundColor: (isLoading || !isValidCode) ? '#9ca3af' : '#2563eb', cursor: (isLoading || !isValidCode) ? 'not-allowed' : 'pointer' }"
        >
          <span v-if="isLoading">Claiming...</span>
          <span v-else>Claim</span>
        </button>
      </form>

      <p v-if="!isValidCode && deviceCode.length > 0" style="font-size: 0.75rem; color: #dc2626; margin-bottom: 1rem;">Code must be 4 digits.</p>

      <div v-if="message" role="status" aria-live="polite">
        <div
          :style="{
            padding: '0.5rem 1rem',
            borderRadius: '0.375rem',
            fontSize: '0.875rem',
            textAlign: 'center',
            border: '1px solid',
            backgroundColor: messageType === 'success' ? '#f0fdf4' : '#fef2f2',
            color: messageType === 'success' ? '#166534' : '#991b1b',
            borderColor: messageType === 'success' ? '#bbf7d0' : '#fecaca'
          }"
        >
          {{ message }}
        </div>
      </div>
    </div>
  </Section>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

const apiClient = createApiClient()

const deviceCode = ref('')
const isLoading = ref(false)
const message = ref('')
const messageType = ref<'success' | 'error'>('success')

const isValidCode = computed(() => {
  return /^\d{4}$/.test(deviceCode.value)
})

const claimDeviceCode = async () => {
  if (!isValidCode.value) return

  isLoading.value = true
  message.value = ''

  try {
    const response = await apiClient.claimDeviceCode({ code: deviceCode.value })

    if (response.success) {
      message.value = 'Device code claimed successfully! The other device should now be authenticated.'
      messageType.value = 'success'
      deviceCode.value = ''
    } else {
      message.value = response.message || 'Failed to claim device code'
      messageType.value = 'error'
    }
  } catch (err) {
    message.value = 'Error claiming device code. Please try again.'
    messageType.value = 'error'
    console.error('Failed to claim device code:', err)
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
</style>
