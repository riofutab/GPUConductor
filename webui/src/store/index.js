import { defineStore } from 'pinia'
import { api } from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('token') || '',
    stats: null,
    tasks: { tasks: [], total: 0 },
    nodes: [],
    users: []
  }),
  getters: {
    isAuthenticated: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin'
  },
  actions: {
    async login(payload) {
      const data = await api.login(payload)
      this.token = data.token
      this.user = data.user
      localStorage.setItem('token', data.token)
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('token')
    },
    async fetchProfile() {
      this.user = await api.profile()
    },
    async fetchStats() {
      this.stats = await api.stats()
    },
    async fetchTasks(params = { page: 1, pageSize: 20 }) {
      this.tasks = await api.tasks(params)
    },
    async createTask(task) {
      await api.createTask(task)
      await this.fetchTasks()
    },
    async cancelTask(id) {
      await api.cancelTask(id)
      await this.fetchTasks()
    },
    async fetchNodes() {
      this.nodes = await api.nodes()
    },
    async fetchUsers() {
      this.users = await api.users()
    },
    async updateUserRole(id, role) {
      await api.updateUserRole(id, role)
      await this.fetchUsers()
    }
  }
})
