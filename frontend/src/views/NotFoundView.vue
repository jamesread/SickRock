<script setup lang="ts">
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { HomeIcon, ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const router = useRouter()

function goHome() {
  router.push('/')
}

function goBack() {
  router.go(-1)
}
</script>

<template>
  <section class="not-found">
    <div class="not-found-content">
      <div class="error-code">404</div>
      <h1>Page Not Found</h1>
      <p>The page you're looking for doesn't exist or has been moved.</p>

      <div class="actions">
        <button @click="goBack" class="button neutral">
          <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
          Go Back
        </button>
        <button @click="goHome" class="button good">
          <HugeiconsIcon :icon="HomeIcon" width="16" height="16" />
          Go Home
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  padding: 2rem;
}

.not-found-content {
  text-align: center;
  max-width: 500px;
}

.error-code {
  font-size: 8rem;
  font-weight: bold;
  color: #e0e0e0;
  line-height: 1;
  margin-bottom: 1rem;
}

h1 {
  font-size: 2rem;
  margin: 0 0 1rem 0;
  color: #333;
}

p {
  font-size: 1.1rem;
  color: #666;
  margin: 0 0 2rem 0;
  line-height: 1.5;
}

.actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}


@media (max-width: 768px) {
  .error-code {
    font-size: 6rem;
  }

  h1 {
    font-size: 1.5rem;
  }

  p {
    font-size: 1rem;
  }

  .actions {
    flex-direction: column;
    align-items: center;
  }

}
</style>
