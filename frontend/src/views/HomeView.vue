<script setup lang="ts">
import { ref } from 'vue'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'

const message = ref('Loading...')
// Transport handled by authenticated client
const client = createApiClient()

async function ping() {
  try {
    const res = await client.ping({ message: 'ping' })
    message.value = `Server says: ${res.message} @ ${Number(res.timestampUnix)}`
  } catch (err) {
    message.value = `Error: ${String(err)}`
  }
}

ping()
</script>

<template>
  <section>
    <h2>Home</h2>
    <button @click="ping">Ping</button>
    <pre>{{ message }}</pre>
  </section>
</template>


