export const blockTypes = [
  { type: 'text', label: 'Text', icon: 'T', description: 'Plain text for your ideas', keywords: 'paragraph plain' },
  { type: 'heading', label: 'Heading', icon: 'H', description: 'Give your section a title', keywords: 'title section h1' },
  { type: 'todo', label: 'To-do list', icon: '☑', description: 'Keep track of a task', keywords: 'task checkbox check todo' },
  { type: 'bullet', label: 'Bulleted list', icon: '•', description: 'Organize a simple list', keywords: 'bullet list unordered' },
]

export function slashQuery(text, caret) {
  if (caret !== text.length || !/^\/[^\s/]*$/.test(text)) return null
  return text.slice(1)
}

export function filterBlockTypes(query = '') {
  const value = query.trim().toLowerCase()
  return blockTypes.filter((item) => `${item.label} ${item.keywords}`.toLowerCase().includes(value))
}

export function convertBlock(block, type, removeCommand = false) {
  if (!blockTypes.some((item) => item.type === type)) return block
  return { ...block, type,
    text: removeCommand && slashQuery(block.text || '', (block.text || '').length) !== null ? '' : block.text,
    ...(type === 'todo' ? { checked: block.type === 'todo' ? Boolean(block.checked) : false } : {}),
  }
}

export function nextBlock(block, text = '') {
  return { id: crypto.randomUUID(), type: ['todo', 'bullet'].includes(block.type) ? block.type : 'text',
    text, ...(block.type === 'todo' ? { checked: false } : {}) }
}