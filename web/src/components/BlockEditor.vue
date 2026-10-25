<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, onUpdated, ref } from 'vue'
import { blockTypes, convertBlock, filterBlockTypes, nextBlock, slashQuery } from '../lib/blocks.js'

const props = defineProps({ blocks: { type: Array, required: true } })
const emit = defineEmits(['update:blocks'])
const editor = ref(null)
const menu = ref(null)
const activeOption = ref(0)
const commands = computed(() => filterBlockTypes(menu.value?.query || ''))
const menuID = `block-menu-${crypto.randomUUID()}`

function resize() {
  editor.value?.querySelectorAll('textarea').forEach((input) => {
    input.style.height = 'auto'
    input.style.height = `${input.scrollHeight}px`
  })
}

function focusBlock(index, caret) {
  nextTick(() => {
    const input = editor.value?.querySelector(`[data-block-index="${index}"]`)
    input?.focus()
    if (input && caret !== undefined) input.setSelectionRange(caret, caret)
  })
}

function openMenu(index, source, query = '') {
  menu.value = { id: props.blocks[index].id, source, query }
  activeOption.value = 0
}

function input(event, index) {
  const text = event.target.value
  update(index, { text })
  const query = slashQuery(text, event.target.selectionStart)
  if (!event.isComposing && query !== null) openMenu(index, 'slash', query)
  else menu.value = null
}

function choose(command) {
  if (!command || !menu.value) return
  const index = props.blocks.findIndex((block) => block.id === menu.value.id)
  if (index < 0) return
  update(index, convertBlock(props.blocks[index], command.type, menu.value.source === 'slash'))
  menu.value = null
  focusBlock(index)
}

function update(index, patch) {
  const blocks = props.blocks.map((block, i) => i === index ? { ...block, ...patch } : block)
  emit('update:blocks', blocks)
}

function addAfter(index) {
  const blocks = [...props.blocks]
  blocks.splice(index + 1, 0, { id: crypto.randomUUID(), type: 'text', text: '' })
  emit('update:blocks', blocks)
  menu.value = null
  focusBlock(index + 1)
}

function keydown(event, index) {
  if (event.isComposing || event.keyCode === 229) return
  if (menu.value?.id === props.blocks[index].id) {
    if (event.key === 'Escape') { event.preventDefault(); menu.value = null; return }
    if (['ArrowDown', 'ArrowUp'].includes(event.key)) {
      event.preventDefault()
      const count = commands.value.length
      if (count) activeOption.value = (activeOption.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
      return
    }
    if (event.key === 'Enter' && !event.shiftKey && commands.value.length) {
      event.preventDefault()
      choose(commands.value[activeOption.value])
      return
    }
  }
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    const block = props.blocks[index]
    if (!block.text && ['todo', 'bullet'].includes(block.type)) {
      update(index, { type: 'text' })
      menu.value = null
      return
    }
    const start = event.target.selectionStart
    const end = event.target.selectionEnd
    const blocks = [...props.blocks]
    blocks[index] = { ...block, text: block.text.slice(0, start) }
    blocks.splice(index + 1, 0, nextBlock(block, block.text.slice(end)))
    emit('update:blocks', blocks)
    menu.value = null
    focusBlock(index + 1, 0)
  }
  if (event.key === 'Backspace' && !props.blocks[index].text && props.blocks.length > 1) {
    event.preventDefault()
    const blocks = props.blocks.filter((_, i) => i !== index)
    emit('update:blocks', blocks)
    menu.value = null
    focusBlock(Math.max(0, index - 1))
  }
}

function outside(event) {
  if (!editor.value?.contains(event.target)) menu.value = null
}

onMounted(() => {
  resize()
  document.addEventListener('pointerdown', outside)
  window.addEventListener('resize', resize)
})
onUpdated(resize)
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', outside)
  window.removeEventListener('resize', resize)
})
</script>

<template>
  <div ref="editor" class="block-editor" @focusout="(event) => { if (!editor.contains(event.relatedTarget)) menu = null }">
    <p class="editor-hint">Type <kbd>/</kbd> for blocks · Shift + Enter for a new line</p>
    <div v-for="(block, index) in blocks" :key="block.id" class="block" :class="`block-${block.type}`">
      <button class="block-handle" :aria-label="`Change block ${index + 1} type`" aria-haspopup="listbox" :aria-expanded="menu?.id === block.id" title="Change block type" @click="menu?.id === block.id ? menu = null : openMenu(index, 'handle')" @keydown="menu?.id === block.id && ['ArrowDown', 'ArrowUp', 'Escape', 'Enter'].includes($event.key) && keydown($event, index)">⋮⋮</button>
      <input v-if="block.type === 'todo'" class="todo-check" type="checkbox" :aria-label="`Complete task ${index + 1}`" :checked="block.checked" @change="update(index, { checked: $event.target.checked })" />
      <span v-if="block.type === 'bullet'" class="bullet">•</span>
      <textarea
        :data-block-index="index"
        :value="block.text"
        :aria-label="`${blockTypes.find((item) => item.type === block.type)?.label || 'Text'} block ${index + 1}`"
        :aria-controls="menu?.id === block.id ? menuID : undefined"
        :aria-activedescendant="menu?.id === block.id && commands.length ? `${menuID}-${activeOption}` : undefined"
        :placeholder="block.type === 'heading' ? 'Section title' : 'Write something, or type / for blocks…'"
        rows="1"
        @input="input($event, index)"
        @keydown="keydown($event, index)"
        @focus="() => { if (menu?.id !== block.id) menu = null }"
        @click="menu = null"
      />
      <div v-if="menu?.id === block.id" class="block-menu" @keydown.esc.prevent.stop="menu = null; focusBlock(index)">
        <p class="block-menu-label">{{ menu.source === 'slash' ? 'Insert a block' : 'Turn into' }}</p>
        <div :id="menuID" role="listbox" aria-label="Block type">
          <button v-for="(command, option) in commands" :id="`${menuID}-${option}`" :key="command.type" role="option" :aria-selected="option === activeOption" :class="{ highlighted: option === activeOption }" @pointerdown.prevent @click="choose(command)" @mouseenter="activeOption = option">
            <span class="block-menu-icon" aria-hidden="true">{{ command.icon }}</span>
            <span><strong>{{ command.label }}</strong><small>{{ command.description }}</small></span>
          </button>
          <p v-if="!commands.length" class="block-menu-empty">No blocks found. Keep typing or press Escape.</p>
        </div>
        <p class="block-menu-footer">↑↓ to browse · Enter to select · Esc to close</p>
      </div>
    </div>
    <button class="add-block" @click="addAfter(blocks.length - 1)">＋ Add block</button>
  </div>
</template>
