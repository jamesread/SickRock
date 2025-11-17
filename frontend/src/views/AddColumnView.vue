<script setup lang="ts">
import { useRoute } from 'vue-router'
import AddColumn from '../components/AddColumn.vue'

const route = useRoute()
const tableId = route.params.tableName as string

// Get query parameters for pre-filling the form
const initialName = route.query.name as string | undefined
const initialType = route.query.type as 'string' | 'int64' | 'tinyint' | 'datetime' | undefined
</script>

<template>
  <section class = "with-header-and-content">
    <div class = "section-header">
      <h2>Add Column</h2>
    </div>
    <div class = "section-content padding">
      <AddColumn
        :table-id="tableId"
        :initial-name="initialName"
        :initial-type="initialType"
      />
    </div>
  </section>
</template>
