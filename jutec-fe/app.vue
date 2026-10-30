<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>

<script setup lang="ts">
// Aplikasi selalu gelap — kunci class `dark` sejak SSR agar komponen Nuxt UI tidak memakai varian terang.
useColorMode().preference = 'dark'
useHead({ htmlAttrs: { class: 'dark' } })
</script>
