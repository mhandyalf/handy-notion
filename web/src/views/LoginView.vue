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
  <AuthShell eyebrow="Selamat datang kembali" title="Masuk ke ruangmu" subtitle="Lanjutkan dari tempat terakhir kamu berhenti.">
    <form class="auth-form" @submit.prevent="submit">
      <label>Username<input v-model.trim="username" required autocomplete="username" placeholder="username kamu" /></label>
      <label>Password<input v-model="password" required type="password" autocomplete="current-password" placeholder="••••••••" /></label>
      <p v-if="error" class="form-error">{{ error }}</p>
      <button class="primary-button" :disabled="busy">{{ busy ? 'Sebentar…' : 'Masuk' }}</button>
    </form>
    <p class="auth-switch">Belum punya akun? <RouterLink to="/register">Buat akun</RouterLink></p>
  </AuthShell>
</template>

