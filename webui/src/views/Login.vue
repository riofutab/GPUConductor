<template>
  <div class="login-wrapper">
    <div class="card login-card">
      <h2>GPUConductor</h2>
      <p>统一的 GPU 调度平台</p>
      <form @submit.prevent="handleLogin">
        <label>手机号</label>
        <input type="text" v-model="form.mobile" placeholder="输入手机号" required>
        <label>密码</label>
        <input type="password" v-model="form.password" placeholder="输入密码" required>
        <button class="btn-primary" type="submit" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
        <p class="error" v-if="error">{{ error }}</p>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../store'

const form = ref({ mobile: '', password: '' })
const loading = ref(false)
const error = ref('')
const router = useRouter()
const auth = useAuthStore()

const handleLogin = async () => {
  loading.value = true
  error.value = ''
  try {
    await auth.login(form.value)
    await auth.fetchProfile()
    router.push('/')
  } catch (err) {
    error.value = err.response?.data?.error || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  min-height: calc(100vh - 80px);
  display: flex;
  align-items: center;
  justify-content: center;
}
.login-card {
  max-width: 360px;
  text-align: center;
}
.login-card h2 {
  margin-bottom: 0.35rem;
}
.login-card form {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  margin-top: 1rem;
}
.error {
  color: #f87171;
}
</style>
