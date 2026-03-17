/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL for the Relicta API server (e.g., http://localhost:8080). */
  readonly VITE_API_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
