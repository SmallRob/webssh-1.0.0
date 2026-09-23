<template>
  <div class="rdp-page">
    <RdpConsole />
  </div>
</template>
<script>
import RdpConsole from '@/components/RdpConsole.vue'
import connFromQuery from '@/mixins/connFromQuery'

export default {
  name: 'RdpPage',
  components: { RdpConsole },
  mixins: [connFromQuery('rdp')]
}
</script>
<style scoped>
.rdp-page { height: 100vh; background: var(--bg-color); }
</style>
