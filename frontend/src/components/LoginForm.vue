<template>
  <section>

      <h2>
        <img :src="logo" alt="SickRock" class="logo icon" width = "32" style = "display: inline-block; vertical-align: sub;"/>
        SickRock
      </h2>

      <form @submit.prevent="handleLogin" class="login-form">
          <input
            id="username"
            v-model="username"
            type="text"
            required
            :disabled="isLoading"
            placeholder="Enter username"
          />

          <input
            id="password"
            v-model="password"
            type="password"
            required
            :disabled="isLoading"
            placeholder="Enter password"
          />

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <button
          type="submit"
          :disabled="isLoading || !username || !password"
          class="login-button"
        >
          <span v-if="isLoading">Signing in...</span>
          <span v-else>Login</span>
        </button>
      </form>
    </section>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import logo from '../resources/images/logo.png'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const isLoading = computed(() => authStore.isLoading)
const error = computed(() => authStore.error)

// Redirect to home if already authenticated (runs whenever authentication state changes)
watch(() => authStore.isAuthenticated, (isAuthenticated) => {
  if (isAuthenticated) {
    router.push('/')
  }
}, { immediate: true })

const handleLogin = async () => {
  const success = await authStore.login(username.value, password.value)
  if (success) {
    router.push('/')
  }
}
</script>

<style scoped>
h2 {
  text-align: center;
  padding-bottom: 20px;
  font-size: 1.5em;
}

section {
  max-width: 400px;
  margin: 10em auto;
}

button:disabled {
  background: #ccc;
  cursor: not-allowed;
  opacity: 0.6;
}

.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.login-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  padding: 40px;
  width: 100%;
  max-width: 400px;
}

.login-form {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

.error-message {
  background-color: #fee;
  color: #c33;
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid #fcc;
  font-size: 0.9rem;
}

</style>
