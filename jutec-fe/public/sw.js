// Jutec tidak memakai service worker. File ini hanya menonaktifkan service worker lama
// (dari proyek lain yang pernah berjalan di localhost:3000) agar tidak terus meminta /sw.js.
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', async () => {
  await self.registration.unregister()
  const clients = await self.clients.matchAll({ type: 'window' })
  clients.forEach(c => c.navigate(c.url))
})
