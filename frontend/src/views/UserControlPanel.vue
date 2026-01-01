<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { UserIcon, BookmarkIcon, SettingsIcon, KeyIcon, NotificationIcon, Download01Icon } from '@hugeicons/core-free-icons'

const authStore = useAuthStore()
const router = useRouter()
const user = computed(() => authStore.user)

function navigateTo(path: string) {
  router.push(path)
}
</script>

<template>
  <Section title="User Control Panel">
    <div class="control-panel-container">
      <div class="user-welcome">
        <h2>Welcome, {{ user?.username }}</h2>
        <p class="welcome-message">Manage your account settings and preferences</p>
      </div>

      <div class="control-panel-grid">
        <!-- User Preferences Card -->
        <div class="control-panel-card" @click="navigateTo('/user-preferences')">
          <div class="card-icon">
            <HugeiconsIcon :icon="SettingsIcon" />
          </div>
          <div class="card-content">
            <h3>User Preferences</h3>
            <p>Manage your account settings, theme, language, and preferences</p>
          </div>
          <div class="card-arrow">
            →
          </div>
        </div>

        <!-- Bookmarks Card -->
        <div class="control-panel-card" @click="navigateTo('/user-bookmarks')">
          <div class="card-icon">
            <HugeiconsIcon :icon="BookmarkIcon" />
          </div>
          <div class="card-content">
            <h3>Bookmarks</h3>
            <p>View and manage your saved bookmarks</p>
          </div>
          <div class="card-arrow">
            →
          </div>
        </div>

        <!-- API Keys Card -->
        <div class="control-panel-card" @click="navigateTo('/user-api-keys')">
          <div class="card-icon">
            <HugeiconsIcon :icon="KeyIcon" />
          </div>
          <div class="card-content">
            <h3>API Keys</h3>
            <p>Create and manage your API keys for programmatic access</p>
          </div>
          <div class="card-arrow">
            →
          </div>
        </div>

        <!-- Notifications Card -->
        <div class="control-panel-card" @click="navigateTo('/user-notifications')">
          <div class="card-icon">
            <HugeiconsIcon :icon="NotificationIcon" />
          </div>
          <div class="card-content">
            <h3>Notifications</h3>
            <p>Configure notification channels and event subscriptions</p>
          </div>
          <div class="card-arrow">
            →
          </div>
        </div>

        <!-- PWA & Service Worker Card -->
        <div class="control-panel-card" @click="navigateTo('/admin/pwa-installation')">
          <div class="card-icon">
            <HugeiconsIcon :icon="Download01Icon" />
          </div>
          <div class="card-content">
            <h3>PWA & Service Worker</h3>
            <p>Manage Progressive Web App installation and service worker</p>
          </div>
          <div class="card-arrow">
            →
          </div>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.control-panel-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.user-welcome {
  margin-bottom: 2rem;
  text-align: center;
}

.user-welcome h2 {
  margin: 0 0 0.5rem 0;
  color: #212529;
  font-size: 1.75rem;
  font-weight: 600;
}

.welcome-message {
  margin: 0;
  color: #6c757d;
  font-size: 1rem;
}

.control-panel-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1.5rem;
  margin-top: 2rem;
}

.control-panel-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding: 1.5rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  min-height: 160px;
}

.control-panel-card:hover {
  border-color: #007bff;
  box-shadow: 0 4px 12px rgba(0, 123, 255, 0.15);
  transform: translateY(-2px);
}

.card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  background: #f8f9fa;
  border-radius: 8px;
  margin-bottom: 1rem;
  color: #007bff;
}

.card-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.card-content {
  flex: 1;
}

.card-content h3 {
  margin: 0 0 0.5rem 0;
  color: #212529;
  font-size: 1.125rem;
  font-weight: 600;
}

.card-content p {
  margin: 0;
  color: #6c757d;
  font-size: 0.9rem;
  line-height: 1.5;
}

.card-arrow {
  position: absolute;
  top: 1.5rem;
  right: 1.5rem;
  color: #6c757d;
  font-size: 1.25rem;
  font-weight: 300;
  transition: all 0.2s ease;
}

.control-panel-card:hover .card-arrow {
  color: #007bff;
  transform: translateX(4px);
}

/* Responsive design */
@media (max-width: 768px) {
  .control-panel-container {
    padding: 1rem;
  }

  .control-panel-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }

  .user-welcome h2 {
    font-size: 1.5rem;
  }
}
</style>
