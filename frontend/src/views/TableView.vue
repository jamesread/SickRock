<script setup lang="ts">
import Table from '../components/Table.vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const tableId = route.params.tableName as string
</script>

<template>
  <Table :table-id="tableId" />
  
</template>


