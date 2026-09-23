import Vue from 'vue'
import Router from 'vue-router'
import Login from '@/views/Login.vue'
import TerminalPage from '@/views/TerminalPage.vue'
import RdpPage from '@/views/RdpPage.vue'
import VncPage from '@/views/VncPage.vue'

Vue.use(Router)

export default new Router({
  mode: 'history',
  routes: [
    { path: '/', component: Login },
    { path: '/terminal', component: TerminalPage },
    // 图形化远程桌面：RDP 走 RDCleanPath 代理，VNC 走 RFB 字节中继
    { path: '/rdp', component: RdpPage },
    { path: '/vnc', component: VncPage }
  ]
})
