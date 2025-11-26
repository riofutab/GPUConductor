<template>
  <div class="card">
    <div class="card-header">
      <div>
        <h3>LDAP 配置</h3>
        <p class="muted small-text">保存后立即生效，用于登录认证。</p>
      </div>
      <button class="btn-ghost btn-small" type="button" @click="loadLdap" :disabled="ldapLoading">
        {{ ldapLoading ? '刷新中...' : '刷新' }}
      </button>
    </div>
    <form class="settings-form" @submit.prevent="saveLdap">
      <div class="two-col">
        <div>
          <label>Host</label>
          <input v-model="ldapForm.host" placeholder="ldap.example.com" required />
        </div>
        <div>
          <label>Port</label>
          <input v-model.number="ldapForm.port" type="number" min="1" placeholder="389" />
        </div>
      </div>
      <div>
        <label>Base DN</label>
        <input v-model="ldapForm.base_dn" placeholder="dc=example,dc=com" />
      </div>
      <div>
        <label>User DN 模板</label>
        <input v-model="ldapForm.user_dn" placeholder="uid=%s,ou=people,dc=example,dc=com" />
      </div>
      <div>
        <label>Bind DN</label>
        <input v-model="ldapForm.bind_dn" placeholder="cn=admin,dc=example,dc=com" />
      </div>
      <div>
        <label>Bind Password</label>
        <input v-model="ldapForm.bind_pass" type="password" placeholder="可留空不改" />
        <p class="muted small-text">允许编辑密码，留空表示不修改。</p>
      </div>
      <div>
        <label>User Filter</label>
        <input v-model="ldapForm.user_filter" placeholder="(uid=%s)" />
      </div>
      <div class="actions">
        <button class="btn-primary" type="submit" :disabled="ldapSaving">
          {{ ldapSaving ? '保存中...' : '保存' }}
        </button>
        <span class="muted small-text" v-if="ldap.password_set">已配置密码</span>
      </div>
      <p class="error" v-if="ldapError">{{ ldapError }}</p>
      <p class="muted" v-if="ldapSuccess">{{ ldapSuccess }}</p>
    </form>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'

const ldapLoading = ref(false)
const ldapSaving = ref(false)
const ldapError = ref('')
const ldapSuccess = ref('')
const ldap = reactive({ password_set: false })
const ldapForm = reactive({
  host: '',
  port: 389,
  base_dn: '',
  user_dn: '',
  bind_dn: '',
  bind_pass: '',
  user_filter: ''
})

const loadLdap = async () => {
  ldapLoading.value = true
  ldapError.value = ''
  ldapSuccess.value = ''
  try {
    const data = await api.ldapSettings()
    Object.assign(ldapForm, {
      host: data.host || '',
      port: data.port || 389,
      base_dn: data.base_dn || '',
      user_dn: data.user_dn || '',
      bind_dn: data.bind_dn || '',
      bind_pass: '',
      user_filter: data.user_filter || ''
    })
    ldap.password_set = !!data.password_set
  } catch (err) {
    ldapError.value = err.response?.data?.error || '获取LDAP配置失败（需要管理员权限）'
  } finally {
    ldapLoading.value = false
  }
}

const saveLdap = async () => {
  ldapSaving.value = true
  ldapError.value = ''
  ldapSuccess.value = ''
  try {
    await api.updateLdapSettings({ ...ldapForm })
    ldapSuccess.value = '保存成功'
    ldapForm.bind_pass = ''
    await loadLdap()
  } catch (err) {
    ldapError.value = err.response?.data?.error || '保存失败'
  } finally {
    ldapSaving.value = false
  }
}

onMounted(loadLdap)
</script>

<style scoped>
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}
.actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.small-text {
  font-size: 0.9rem;
}
.error {
  color: #f87171;
}
</style>
