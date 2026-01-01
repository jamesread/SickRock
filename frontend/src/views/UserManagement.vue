<script setup lang="ts">
import { ref } from 'vue'
import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

const client = createApiClient()
const resetUser = ref({ username: '', newPassword: '' })
const resetStatus = ref<string>('')
const error = ref<string | null>(null)

async function resetUserPassword() {
  resetStatus.value = ''
  error.value = null
  try {
    if (!resetUser.value.username || !resetUser.value.newPassword) {
      error.value = 'Username and new password are required'
      return
    }
    const resp = await client.resetUserPassword({
      username: resetUser.value.username,
      newPassword: resetUser.value.newPassword
    } as any)
    if ((resp as any).success) {
      resetStatus.value = 'Password updated successfully'
      resetUser.value = { username: '', newPassword: '' }
    } else {
      error.value = (resp as any).message || 'Failed to update password'
    }
  } catch (e) {
    console.error(e)
    error.value = 'Network error'
  }
}
</script>

<template>
  <Section title="User Management">
    <div class="user-management-container">
      <div class="user-management-section">
        <h2>Reset User Password</h2>
        <p class="section-description">Reset a user's password by providing their username and a new password.</p>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <div v-if="resetStatus" class="success-message">
          {{ resetStatus }}
        </div>

        <div class="reset-password-form">
          <div class="form-grid">
            <div class="form-group">
              <label for="username">Username</label>
              <input
                id="username"
                v-model="resetUser.username"
                type="text"
                placeholder="e.g., admin"
              />
            </div>
            <div class="form-group">
              <label for="newPassword">New Password</label>
              <input
                id="newPassword"
                v-model="resetUser.newPassword"
                type="password"
                placeholder="Enter new password"
              />
            </div>
          </div>
          <div class="form-actions">
            <button @click="resetUserPassword" class="save-btn">Reset Password</button>
          </div>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.user-management-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.user-management-section {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  border: 1px solid #e9ecef;
}

.user-management-section h2 {
  margin: 0 0 0.5rem 0;
  color: #212529;
  font-size: 1.5rem;
  font-weight: 600;
}

.section-description {
  margin: 0 0 2rem 0;
  color: #6c757d;
  font-size: 0.95rem;
}

.error-message {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  margin-bottom: 1.5rem;
  border: 1px solid #f5c6cb;
}

.success-message {
  background: #d4edda;
  color: #155724;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  margin-bottom: 1.5rem;
  border: 1px solid #c3e6cb;
}

.reset-password-form {
  background: #f8f9fa;
  padding: 1.5rem;
  border-radius: 8px;
  border: 1px solid #e9ecef;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5rem;
  margin-bottom: 1.5rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-weight: 500;
  color: #333;
  font-size: 0.9rem;
}

.form-group input {
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  transition: border-color 0.2s ease;
}

.form-group input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 3px rgba(0, 123, 255, 0.1);
}

.form-actions {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.save-btn {
  background: #28a745;
  color: white;
  border: none;
  padding: 0.75rem 1.5rem;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  font-size: 1rem;
  transition: background-color 0.2s ease;
}

.save-btn:hover {
  background: #218838;
}

.save-btn:active {
  background: #1e7e34;
}

@media (max-width: 768px) {
  .user-management-container {
    padding: 1rem;
  }

  .user-management-section {
    padding: 1.5rem;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
