<script setup>
import { nextTick } from 'vue'

const props = defineProps({ blocks: { type: Array, required: true } })
const emit = defineEmits(['update:blocks'])

function update(index, patch) {
  const blocks = props.blocks.map((block, i) => i === index ? { ...block, ...patch } : block)
  emit('update:blocks', blocks)
}

function addAfter(index) {
  const blocks = [...props.blocks]
  blocks.splice(index + 1, 0, { id: crypto.randomUUID(), type: 'text', text: '' })
  emit('update:blocks', blocks)
  nextTick(() => document.querySelector(`[data-block-index="${index + 1}"]`)?.focus())
}

function keydown(event, index) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    addAfter(index)
  }
  if (event.key === 'Backspace' && !props.blocks[index].text && props.blocks.length > 1) {
    event.preventDefault()
    const blocks = props.blocks.filter((_, i) => i !== index)
    emit('update:blocks', blocks)
    nextTick(() => document.querySelector(`[data-block-index="${Math.max(0, index - 1)}"]`)?.focus())
  }
}

function cycleType(index) {
  const types = ['text', 'heading', 'todo', 'bullet']
  const current = types.indexOf(props.blocks[index].type)
  update(index, { type: types[(current + 1) % types.length] })
}
</script>

<template>
  <div class="block-editor">
    <div v-for="(block, index) in blocks" :key="block.id" class="block" :class="`block-${block.type}`">
      <button class="block-handle" :title="`Ubah tipe: ${block.type}`" @click="cycleType(index)">⋮⋮</button>
      <input v-if="block.type === 'todo'" class="todo-check" type="checkbox" :checked="block.checked" @change="update(index, { checked: $event.target.checked })" />
      <span v-if="block.type === 'bullet'" class="bullet">•</span>
      <textarea
        :data-block-index="index"
        :value="block.text"
        :placeholder="block.type === 'heading' ? 'Judul bagian' : index === 0 ? 'Mulai menulis…' : ''"
        rows="1"
        @input="update(index, { text: $event.target.value }); $event.target.style.height = 'auto'; $event.target.style.height = `${$event.target.scrollHeight}px`"
        @keydown="keydown($event, index)"
      />
    </div>
    <button class="add-block" @click="addAfter(blocks.length - 1)">＋ Tambah blok</button>
  </div>
</template>

