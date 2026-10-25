<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { api } from '../lib/api.js'

const props = defineProps({ noteId: { type: String, required: true }, flush: { type: Function, required: true }, blocked: Boolean })
const open = ref(false)
const token = ref(null)
const pending = ref(false)
const ready = ref(false)
const error = ref('')
const copied = ref(false)
let generation = 0
const link = computed(() => token.value ? `${window.location.origin}/s/${token.value}` : '')
watch(() => props.noteId, () => { generation++; open.value = false; token.value = null; ready.value = false; pending.value = false; error.value = ''; copied.value = false })
onBeforeUnmount(() => { generation++ })

async function operation(action) {
  if (pending.value) return
  const current = generation
  const id = props.noteId
  pending.value = true
  error.value = ''
  copied.value = false
  try {
    const result = await action(id)
    if (current !== generation) return
    token.value = result?.token || null
    ready.value = true
  } catch (err) {
    if (current === generation) error.value = err.message
  } finally {
    if (current === generation) pending.value = false
  }
}
function show() {
  open.value = !open.value
  if (open.value) { ready.value = false; void operation(api.getShare) }
}
async function enable(id) {
  if (props.blocked) throw new Error('Resolve any conflict and restore the note before enabling sharing.')
  if (!await props.flush()) throw new Error('Save this note and resolve any conflict before enabling sharing.')
  if (props.noteId !== id || props.blocked) throw new Error('The selected note changed. Open sharing again.')
  return api.enableShare(id)
}
function revoke() {
  if (window.confirm('Revoke this public link? Anyone with it will lose access. Previously downloaded copies cannot be recalled.')) {
    void operation(api.revokeShare)
  }
}
async function copy() {
  try { await navigator.clipboard.writeText(link.value); copied.value = true }
  catch { error.value = 'Clipboard unavailable. Select and copy the link manually.' }
}
</script>

<template>
  <div class="note-share">
    <button class="text-button" :aria-expanded="open" @click="show">Share</button>
    <section v-if="open" class="share-panel" aria-label="Public sharing">
      <strong>Share read-only link</strong>
      <p>Anyone with the link can read the latest saved version without signing in. Future saved edits are public too. Local drafts are not shared.</p>
      <p>Archived or deleted notes are unavailable. Restoring an archived note reactivates its link unless revoked.</p>
      <p v-if="error" role="alert">{{ error }}</p>
      <p v-if="pending" role="status">Updating sharing…</p>
      <template v-if="ready && token">
        <label>Public link<input :value="link" readonly @focus="$event.target.select()" /></label>
        <button class="text-button" :disabled="pending" @click="copy">{{ copied ? 'Copied' : 'Copy link' }}</button>
        <button class="text-button" :disabled="pending" @click="revoke">Revoke link</button>
      </template>
      <button v-else-if="ready" class="text-button" :disabled="pending || blocked" @click="operation(enable)">Enable public link</button>
      <p v-if="blocked">Enabling requires a saved, unarchived note without conflicts. Existing links can still be revoked.</p>
      <button class="text-button" @click="open = false">Close</button>
    </section>
  </div>
</template>

<style scoped>
.note-share { position: relative; }
.share-panel { position: absolute; right: 0; top: 100%; z-index: 30; width: min(340px, 80vw); padding: 18px; background: #fff; color: #37352f; border: 1px solid #ddd; border-radius: 8px; box-shadow: 0 8px 28px #0002; font-size: 14px; }
.share-panel p { line-height: 1.5; margin: 12px 0; }
.share-panel input { width: 100%; box-sizing: border-box; padding: 8px; margin: 6px 0; }
</style>