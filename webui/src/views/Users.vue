<template>
  <div class="card">
    <div class="card-header">
      <div>
        <h3>用户管理</h3>
        <p class="muted small-text">仅管理员可见，支持查看用户与调整角色。</p>
      </div>
      <button class="btn-ghost btn-small" type="button" @click="loadUsers" :disabled="loading">
        {{ loading ? '刷新中...' : '刷新' }}
      </button>
    </div>
    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>用户名</th>
            <th>显示名</th>
            <th>手机号</th>
            <th>邮箱</th>
            <th>角色</th>
            <th>最近登录</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>{{ user.username }}</td>
            <td>{{ user.display_name || '-' }}</td>
            <td>{{ user.mobile || '-' }}</td>
            <td>{{ user.email || '-' }}</td>
            <td>
              <select v-model="roles[user.id]" @change="changeRole(user)" :disabled="saving[user.id]">
                <option value="user">普通用户</option>
                <option value="admin">管理员</option>
              </select>
            </td>
            <td>{{ formatTime(user.last_login_at) }}</td>
          </tr>
          <tr v-if="!users.length">
            <td colspan="6" class="muted">暂无用户</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="error" v-if="error">{{ error }}</p>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref, computed } from 'vue'
import { useAuthStore } from '../store'

const auth = useAuthStore()
const loading = ref(false)
const error = ref('')
const saving = reactive({})
const roles = reactive({})

const users = computed(() => auth.users || [])

const loadUsers = async () => {
  loading.value = true
  error.value = ''
  try {
    await auth.fetchUsers()
    users.value.forEach((u) => {
      roles[u.id] = u.role || 'user'
    })
  } catch (err) {
    error.value = err.response?.data?.error || '获取用户失败'
  } finally {
    loading.value = false
  }
}

const changeRole = async (user) => {
  const role = roles[user.id]
  saving[user.id] = true
  error.value = ''
  try {
    await auth.updateUserRole(user.id, role)
  } catch (err) {
    error.value = err.response?.data?.error || '更新角色失败'
    roles[user.id] = user.role // revert
  } finally {
    saving[user.id] = false
    await loadUsers()
  }
}

const formatTime = (value) => (value ? new Date(value).toLocaleString() : '-')

onMounted(loadUsers)
</script>

<style scoped>
.small-text {
  font-size: 0.9rem;
}
.error {
  color: #f87171;
}
select {
  min-width: 140px;
}
</style>
