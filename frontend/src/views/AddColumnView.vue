<script setup lang="ts">
import { useRoute } from 'vue-router'
import AddColumn from '../components/AddColumn.vue'

const route = useRoute()
const tableId = route.params.tableName as string
</script>

<template>
  <section class = "with-header-and-content">
    <div class = "section-header">
      <h2>Add Column</h2>
    </div>  
    <div class = "section-content">
      <AddColumn :table-id="tableId" />
    </div>
  </section>
</template>


