import { createRouter, createWebHistory } from 'vue-router'
import { auth } from './lib/auth.js'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import WorkspaceView from './views/WorkspaceView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/notes' },
    { path: '/login', component: LoginView, meta: { guest: true } },
    { path: '/register', component: RegisterView, meta: { guest: true } },
    { path: '/notes/:id?', component: WorkspaceView, meta: { auth: true } },
  ],
})

router.beforeEach((to) => {
  const loggedIn = auth.isAuthenticated()
  if (to.meta.auth && !loggedIn) return '/login'
  if (to.meta.guest && loggedIn) return '/notes'
})

export default router

