import axios from 'axios'

const client = axios.create({
  baseURL: '/api/v1'
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const api = {
  login(payload) {
    return client.post('/auth/login', payload).then((res) => res.data)
  },
  profile() {
    return client.get('/users/profile').then((res) => res.data)
  },
  stats() {
    return client.get('/stats').then((res) => res.data)
  },
  tasks(params) {
    return client.get('/tasks', { params }).then((res) => res.data)
  },
  task(id) {
    return client.get(`/tasks/${id}`).then((res) => res.data)
  },
  createTask(payload) {
    return client.post('/tasks', payload).then((res) => res.data)
  },
  cancelTask(id) {
    return client.post(`/tasks/${id}/cancel`).then((res) => res.data)
  },
  taskLogs(id) {
    return client.get(`/tasks/${id}/logs`).then((res) => res.data)
  },
  nodes() {
    return client.get('/nodes').then((res) => res.data)
  },
  users() {
    return client.get('/users').then((res) => res.data)
  },
  updateUserRole(id, role) {
    return client.put(`/users/${id}/role`, { role }).then((res) => res.data)
  },
  ldapSettings() {
    return client.get('/settings/ldap').then((res) => res.data)
  },
  updateLdapSettings(payload) {
    return client.put('/settings/ldap', payload).then((res) => res.data)
  }
}
