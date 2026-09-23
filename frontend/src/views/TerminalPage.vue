<template>
  <div class="terminal-page">
    <Terminal />
  </div>
</template>
<script>
import Terminal from '@/components/Terminal.vue'
export default {
  components: { Terminal },
  beforeCreate() {
    // URL 只允许携带目标信息；凭据统一从 sessionStorage 回退读取，
    // 避免用户名密码出现在链接、浏览器历史与服务端访问日志中
    let { hostname, port, username, command } = this.$route.query;
    if (hostname) hostname = decodeURIComponent(hostname);
    if (username) username = decodeURIComponent(username);
    if (command) command = decodeURIComponent(command);

    let password = '';
    let privateKey = '';
    let passphrase = '';
    const savedInfo = sessionStorage.getItem('sshInfo');
    if (savedInfo) {
      try {
        const info = JSON.parse(savedInfo);
        hostname = info.hostname || hostname;
        port = info.port || port;
        username = info.username || username;
        password = info.password || '';
        command = info.command || command;
        privateKey = info.privateKey || '';
        passphrase = info.passphrase || '';
      } catch (e) { /* 忽略损坏的缓存 */ }
    }

    if (hostname && username && (password || privateKey)) {
      this.$store.commit('SET_SSH', {
        // 终端页始终为 SSH，避免同标签页切换协议后污染 store
        protocol: 'ssh',
        hostname,
        port: Number(port) || 22,
        username,
        password,
        command,
        privateKey,
        passphrase
      });
    }
  }
}
</script>
<style scoped>
.terminal-page { min-height: 100vh; background: var(--bg-color); }
</style> 