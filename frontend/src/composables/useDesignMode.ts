import { ref } from 'vue'

const STORAGE_KEY = 'sickrock.designMode'

function readInitial(): boolean {
  try {
    return sessionStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

const designModeEnabled = ref(readInitial())

function persist(value: boolean) {
  try {
    sessionStorage.setItem(STORAGE_KEY, value ? '1' : '0')
  } catch {
    // ignore
  }
}

/** App-wide “design mode” for table structure / layout editing affordances. */
export function useDesignMode() {
  function setDesignMode(enabled: boolean) {
    designModeEnabled.value = enabled
    persist(enabled)
  }

  function toggleDesignMode() {
    setDesignMode(!designModeEnabled.value)
  }

  return {
    designModeEnabled,
    setDesignMode,
    toggleDesignMode,
  }
}
