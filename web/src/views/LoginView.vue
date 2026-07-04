<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthShell from '../components/AuthShell.vue'
import { api } from '../lib/api.js'
import { auth } from '../lib/auth.js'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const result = await api.login({ username: username.value, password: password.value })
    auth.setToken(result.token)
    router.push('/notes')
  } catch (err) { error.value = err.message }
  finally { busy.value = false }
}
</script>

<template>
  <AuthShell eyebrow="Welcome back" title="Sign in to your space" subtitle="Pick up right where you left off.">
    <form class="auth-form" @submit.prevent="submit">
      <label>Username<input v-model.trim="username" required autocomplete="username" placeholder="your username" /></label>
      <label>Password<input v-model="password" required type="password" autocomplete="current-password" placeholder="••••••••" /></label>
      <p v-if="error" class="form-error">{{ error }}</p>
      <button class="primary-button" :disabled="busy">{{ busy ? 'Signing in…' : 'Sign in' }}</button>
    </form>
    <p class="auth-switch">Do not have an account? <RouterLink to="/register">Create one</RouterLink></p>
  </AuthShell>
</template>
