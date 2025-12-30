import { ref, watch, onMounted } from 'vue'

export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'relicta_theme'

// Global reactive state
const theme = ref<Theme>('system')
const isDark = ref(false)

// Media query for system preference
let mediaQuery: MediaQueryList | null = null

function applyTheme(newTheme: Theme) {
  theme.value = newTheme

  if (newTheme === 'system') {
    localStorage.removeItem(STORAGE_KEY)
    isDark.value = mediaQuery?.matches ?? false
  } else {
    localStorage.setItem(STORAGE_KEY, newTheme)
    isDark.value = newTheme === 'dark'
  }

  // Apply to document
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function initTheme() {
  // Set up media query listener
  if (typeof window !== 'undefined') {
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', (e) => {
      if (theme.value === 'system') {
        isDark.value = e.matches
        if (e.matches) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
      }
    })
  }

  // Load saved preference
  const saved = localStorage.getItem(STORAGE_KEY) as Theme | null
  if (saved && ['light', 'dark', 'system'].includes(saved)) {
    applyTheme(saved)
  } else {
    applyTheme('system')
  }
}

export function useTheme() {
  onMounted(() => {
    // Initialize only once
    if (!mediaQuery) {
      initTheme()
    }
  })

  function setTheme(newTheme: Theme) {
    applyTheme(newTheme)
  }

  function toggleTheme() {
    if (isDark.value) {
      setTheme('light')
    } else {
      setTheme('dark')
    }
  }

  return {
    theme,
    isDark,
    setTheme,
    toggleTheme,
  }
}

// Initialize immediately for SSR-like scenarios
if (typeof window !== 'undefined') {
  // Check for saved theme before Vue hydrates to prevent flash
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'dark' || (saved !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    document.documentElement.classList.add('dark')
  }
}
