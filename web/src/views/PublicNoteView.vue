<script setup>
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../lib/api.js'

const route = useRoute()
const note = ref(null)
const error = ref('')
const loading = ref(true)
let controller
let requestID = 0
const stop = watch(() => route.params.token, async (token) => {
  const id = ++requestID
  controller?.abort()
  controller = new AbortController()
  note.value = null
  error.value = ''
  loading.value = true
  try {
    const result = await api.getPublicNote(token, controller.signal)
    if (id === requestID) note.value = result
  } catch (err) {
    if (id === requestID && err.name !== 'AbortError') error.value = err.message
  } finally {
    if (id === requestID) loading.value = false
  }
}, { immediate: true })
const robots = document.createElement('meta')
robots.name = 'robots'
robots.content = 'noindex,nofollow,noarchive'
document.head.append(robots)
onBeforeUnmount(() => { stop(); requestID++; controller?.abort(); robots.remove() })
</script>

<template>
  <main class="public-note">
    <header><strong>Handy Notes</strong><span>Shared · Read only</span></header>
    <p v-if="loading" role="status">Loading shared note…</p>
    <div v-else-if="error" role="alert"><h1>Note unavailable</h1><p>{{ error }}</p><p>The owner may have revoked the link, archived, or deleted this note.</p></div>
    <article v-else-if="note">
      <div class="public-icon">{{ note.icon }}</div>
      <h1>{{ note.title || 'Untitled' }}</h1>
      <p class="public-meta">Last edited {{ new Date(note.updated_at).toLocaleString() }} · Refresh to see saved updates</p>
      <template v-for="(block, index) in note.content" :key="index">
        <h2 v-if="block?.type === 'heading'" class="public-block">{{ block.text }}</h2>
        <p v-else-if="block?.type === 'todo'" class="public-block"><span :aria-label="block.checked ? 'Completed task' : 'Incomplete task'">{{ block.checked ? '☑' : '☐' }}</span> {{ block.text }}</p>
        <p v-else-if="block?.type === 'bullet'" class="public-block">• {{ block.text }}</p>
        <p v-else class="public-block">{{ block?.text }}</p>
      </template>
    </article>
  </main>
</template>

<style scoped>
.public-note { max-width: 850px; margin: auto; padding: 28px 24px 80px; overflow-wrap: anywhere; }
header { display: flex; justify-content: space-between; gap: 16px; padding-bottom: 40px; }
header span, .public-meta { color: #666; font-size: 14px; }
h1 { font-size: clamp(28px, 5vw, 44px); white-space: pre-wrap; }
.public-icon { font-size: 48px; }
.public-block { white-space: pre-wrap; line-height: 1.7; min-height: 1em; }
</style>