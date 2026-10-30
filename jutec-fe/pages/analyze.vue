<script setup lang="ts">
const route = useRoute()
const url = route.query.url as string
navigateTo(url ? `/discovery?url=${encodeURIComponent(url)}` : '/discovery')
</script>
