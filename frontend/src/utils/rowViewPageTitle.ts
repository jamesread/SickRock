import { ref } from 'vue'

/** Last item row-view heading for breadcrumbs and document title while on a row route. */
export const rowViewPageLabel = ref<string | null>(null)

export function clearRowViewPageLabel() {
  rowViewPageLabel.value = null
}
