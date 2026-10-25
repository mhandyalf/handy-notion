import test from 'node:test'
import assert from 'node:assert/strict'
import { blockTypes, convertBlock, filterBlockTypes, nextBlock, slashQuery } from './blocks.js'

test('slash commands only open at the end of a command-only block', () => {
  assert.equal(slashQuery('/', 1), '')
  assert.equal(slashQuery('/todo', 5), 'todo')
  for (const text of ['Some /text', 'https://example.com', '/two words', '//', 'normal text']) {
    assert.equal(slashQuery(text, text.length), null)
  }
  assert.equal(slashQuery('/heading', 2), null)
})

test('block menu filters labels and aliases without changing supported types', () => {
  assert.equal(filterBlockTypes().length, 4)
  assert.deepEqual(filterBlockTypes('TASK').map((item) => item.type), ['todo'])
  assert.deepEqual(filterBlockTypes('paragraph').map((item) => item.type), ['text'])
  assert.deepEqual(filterBlockTypes('h1').map((item) => item.type), ['heading'])
  assert.equal(filterBlockTypes('unknown').length, 0)
  assert.deepEqual(blockTypes.map((item) => item.type), ['text', 'heading', 'todo', 'bullet'])
})

test('choosing a slash command preserves block identity and clears only command text', () => {
  const block = { id: 'one', type: 'text', text: '/heading' }
  assert.deepEqual(convertBlock(block, 'heading', true), { id: 'one', type: 'heading', text: '' })
  assert.equal(block.text, '/heading')
  assert.equal(convertBlock({ ...block, text: 'Keep /this text' }, 'heading', true).text, 'Keep /this text')
  assert.equal(convertBlock(block, 'unsupported'), block)
})

test('handle conversion preserves content and initializes tasks without stale completion', () => {
  const block = { id: 'one', type: 'text', text: 'Keep this', checked: true }
  assert.deepEqual(convertBlock(block, 'todo'), { ...block, type: 'todo', checked: false })
  assert.equal(convertBlock({ ...block, type: 'todo' }, 'todo').checked, true)
  assert.equal(convertBlock(block, 'bullet').text, 'Keep this')
})

test('new blocks continue lists, reset task completion, and use unique IDs', () => {
  const task = nextBlock({ type: 'todo', checked: true }, 'remaining text')
  assert.equal(task.type, 'todo')
  assert.equal(task.checked, false)
  assert.equal(task.text, 'remaining text')
  assert.equal(nextBlock({ type: 'bullet' }).type, 'bullet')
  assert.equal(nextBlock({ type: 'heading' }).type, 'text')
  assert.notEqual(task.id, nextBlock({ type: 'todo' }).id)
})