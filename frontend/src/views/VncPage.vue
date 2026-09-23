<template>
  <div class="vnc-page">
    <VncConsole />
  </div>
</template>
<script>
import VncConsole from '@/components/VncConsole.vue'
import connFromQuery from '@/mixins/connFromQuery'

export default {
  name: 'VncPage',
  components: { VncConsole },
  mixins: [connFromQuery('vnc')]
}
</script>
<style scoped>
.vnc-page { height: 100vh; background: var(--bg-color); }
</style>
