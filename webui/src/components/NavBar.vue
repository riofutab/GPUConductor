<template>
  <header class="navbar">
    <div class="nav-brand" @click="$router.push('/')">
      <strong>GPUConductor</strong>
    </div>
    <nav class="nav-links" v-if="auth.isAuthenticated">
      <a @click.prevent="$router.push('/')">概览</a>
      <a @click.prevent="$router.push('/tasks')">任务</a>
      <a @click.prevent="$router.push('/nodes')">节点</a>
      <a v-if="auth.isAdmin" @click.prevent="$router.push('/users')">用户</a>
      <a v-if="auth.isAdmin" @click.prevent="$router.push('/settings')">设置</a>
    </nav>
    <div class="nav-right" v-if="auth.isAuthenticated">
      <span class="user-name">{{ auth.user?.display_name || auth.user?.username }}</span>
      <button class="btn-primary" @click="handleLogout">退出</button>
    </div>
  </header>
</template>

<script setup>
import { useAuthStore } from '../store'
const auth = useAuthStore()

const handleLogout = () => {
  auth.logout()
  window.location.href = '/login'
}
</script>

<style scoped>
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
}
.nav-links a {
  margin-right: 1rem;
  color: var(--muted);
  text-decoration: none;
}
.nav-links a:hover {
  color: var(--text);
}
.user-name {
  margin-right: 1rem;
  color: var(--muted);
}
</style>
