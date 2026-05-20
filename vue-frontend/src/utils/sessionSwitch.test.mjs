import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveBaseModelSwitch } from './sessionSwitch.js'

test('switching base model with an active session starts a new session and shows a hint', () => {
  const result = resolveBaseModelSwitch({
    currentSessionId: 'session-1',
    messageCount: 2,
    nextModelType: 'deepseek',
  })

  assert.equal(result.shouldStartNewSession, true)
  assert.equal(result.alertText, '已切换到底座模型 DeepSeek，下一条消息将自动创建新会话。')
})
