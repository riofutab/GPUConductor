import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../store'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import Tasks from '../views/Tasks.vue'
import Nodes from '../views/Nodes.vue'
import Users from '../views/Users.vue'
import Settings from '../views/Settings.vue'

const routes = [
  { path: '/login', name: 'login', component: Login, meta: { public: true } },
  { path: '/', name: 'dashboard', component: Dashboard },
  { path: '/tasks', name: 'tasks', component: Tasks },
  { path: '/nodes', name: 'nodes', component: Nodes },
  { path: '/users', name: 'users', component: Users, meta: { admin: true } },
  { path: '/settings', name: 'settings', component: Settings, meta: { admin: true } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isAuthenticated) {
    next('/login')
  } else if (to.meta.admin && !auth.isAdmin) {
    next('/')
  } else if (to.meta.public && auth.isAuthenticated && to.path === '/login') {
    next('/')
  } else {
    next()
  }
})

export default router
