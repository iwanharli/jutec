export default defineNuxtConfig({
  compatibilityDate: '2024-04-03',
  devtools: { enabled: false },
  // App manifest hanya berguna untuk SSG/ISR; dimatikan agar dev server tidak memunculkan error '#app-manifest'
  experimental: { appManifest: false },
  modules: ['@nuxt/ui', '@vueuse/nuxt'],
  // Warna yang dipakai lewat prop dinamis (:color) — Nuxt UI hanya men-generate class untuk warna di sini
  ui: { safelistColors: ['primary', 'rose', 'amber', 'emerald', 'sky', 'violet', 'cyan', 'purple', 'orange', 'red', 'blue'] },
  colorMode: { preference: 'dark', fallback: 'dark' },
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080',
      wsBase:  process.env.NUXT_PUBLIC_WS_BASE  || 'ws://localhost:8080',
    },
  },
  app: {
    head: {
      title: 'Jutec — Judol Intelligence',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
      link: [
        { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Outfit:wght@500;600;700&family=JetBrains+Mono:wght@400;500&display=swap' }
      ]
    },
  },
})
