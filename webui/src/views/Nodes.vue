<template>
  <div class="card">
    <h3>节点</h3>
    <div class="table-wrapper">
    <table class="table">
      <thead>
        <tr>
          <th>名称</th>
          <th>状态</th>
          <th>地址</th>
          <th>标签</th>
          <th>最近心跳</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="node in auth.nodes" :key="node.id">
          <td>{{ node.name }}</td>
          <td>
            <span :class="['badge', node.status === 'online' ? 'badge-running' : 'badge-failed']">
              {{ node.status }}
            </span>
          </td>
          <td>{{ node.address }}</td>
          <td>{{ node.tags?.join(', ') || '-' }}</td>
          <td>{{ formatTime(node.last_seen) }}</td>
        </tr>
      </tbody>
    </table>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useAuthStore } from '../store'

const auth = useAuthStore()
const formatTime = (value) => value ? new Date(value).toLocaleString() : '-'

onMounted(() => {
  auth.fetchNodes()
})
</script>
