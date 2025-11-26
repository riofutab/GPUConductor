<template>
  <div class="card" v-if="auth.stats">
    <h2>系统概览</h2>
    <div class="stats-grid">
      <div class="stat">
        <span>在线节点</span>
        <strong>{{ auth.stats.nodes.online }} / {{ auth.stats.nodes.total }}</strong>
      </div>
      <div class="stat">
        <span>运行任务</span>
        <strong>{{ auth.stats.tasks.running }}</strong>
      </div>
      <div class="stat">
        <span>等待任务</span>
        <strong>{{ auth.stats.tasks.pending }}</strong>
      </div>
      <div class="stat">
        <span>GPU 总数</span>
        <strong>{{ auth.stats.gpus.total }}</strong>
      </div>
    </div>
  </div>
  <div class="card" v-else>
    <p>加载中...</p>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useAuthStore } from '../store'

const auth = useAuthStore()

onMounted(async () => {
  if (!auth.user) {
    await auth.fetchProfile().catch(() => {})
  }
  await auth.fetchStats()
})
</script>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 1rem;
  margin-top: 1rem;
}
.stat {
  background: rgba(15, 23, 42, 0.45);
  border-radius: 14px;
  padding: 1rem;
}
.stat span {
  color: var(--muted);
  font-size: 0.9rem;
}
.stat strong {
  display: block;
  font-size: 1.8rem;
  margin-top: 0.4rem;
}
</style>
