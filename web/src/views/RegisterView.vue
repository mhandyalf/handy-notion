<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthShell from '../components/AuthShell.vue'
import { api } from '../lib/api.js'

const router = useRouter()
const form = ref({ username: '', email: '', password: '' })
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await api.register(form.value)
    router.push('/login')
  } catch (err) { error.value = err.message }
  finally { busy.value = false }
}
</script>

<template>
  <AuthShell eyebrow="Start writing" title="Create your space" subtitle="One account for every note, plan, and unfinished idea.">
    <form class="auth-form" @submit.prevent="submit">
      <label>Username<input v-model.trim="form.username" required minlength="3" maxlength="50" autocomplete="username" placeholder="choose a username" /></label>
      <label>Email<input v-model.trim="form.email" required type="email" autocomplete="email" placeholder="you@email.com" /></label>
      <label>Password<input v-model="form.password" required minlength="8" type="password" autocomplete="new-password" placeholder="at least 8 characters" /></label>
      <p v-if="error" class="form-error">{{ error }}</p>
      <button class="primary-button" :disabled="busy">{{ busy ? 'Creating…' : 'Create account' }}</button>
    </form>
    <p class="auth-switch">Already have an account? <RouterLink to="/login">Sign in</RouterLink></p>
  </AuthShell>
</template>
