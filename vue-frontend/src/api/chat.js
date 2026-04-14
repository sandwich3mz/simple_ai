import client from './client'

const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1'
const SUCCESS_CODE = 1000

export function getSessions() {
  return client.get('/AI/chat/sessions')
}

export function getHistory(sessionId) {
  return client.post('/AI/chat/history', { sessionId })
}

export function sendMessageNewSession(question, modelType) {
  return client.post('/AI/chat/send-new-session', { question, modelType })
}

export function sendMessage(sessionId, question, modelType) {
  return client.post('/AI/chat/send', { sessionId, question, modelType })
}

function parseEventBlock(rawEvent) {
  const event = { name: 'message', data: '' }
  const lines = rawEvent.split(/\r?\n/)
  const dataLines = []
  for (const line of lines) {
    if (line.startsWith('event:')) {
      event.name = line.slice(6).trim()
      continue
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart())
    }
  }
  event.data = dataLines.join('\n')
  return event
}

function parseJsonSafe(text) {
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function createStreamError(message, statusCode) {
  const error = new Error(message || 'Stream request failed')
  if (statusCode) {
    error.statusCode = statusCode
  }
  return error
}

async function streamRequest({
  endpoint,
  payload,
  token,
  onSessionId,
  onChunk,
  onDone,
}) {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: token ? `Bearer ${token}` : '',
    },
    body: JSON.stringify(payload),
  })

  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    const data = await response.json()
    if (typeof data?.status_code === 'number' && data.status_code !== SUCCESS_CODE) {
      throw createStreamError(data.status_msg, data.status_code)
    }
    throw createStreamError('Stream endpoint returned JSON unexpectedly')
  }

  if (!response.ok || !response.body) {
    throw createStreamError(`Stream request failed with status ${response.status}`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const consumeEvent = (rawEvent) => {
    const { name, data } = parseEventBlock(rawEvent)
    if (!data) {
      return
    }
    if (name === 'error') {
      const parsedError = parseJsonSafe(data)
      const message = parsedError?.message || data
      throw createStreamError(message)
    }
    if (data === '[DONE]') {
      onDone?.()
      return
    }
    const parsedData = parseJsonSafe(data)
    if (parsedData?.sessionId) {
      onSessionId?.(parsedData.sessionId)
      return
    }
    onChunk?.(data)
  }

  while (true) {
    const { value, done } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })
    buffer = buffer.replace(/\r\n/g, '\n')

    let boundary = buffer.indexOf('\n\n')
    while (boundary !== -1) {
      const rawEvent = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      consumeEvent(rawEvent)
      boundary = buffer.indexOf('\n\n')
    }
  }

  if (buffer.trim()) {
    consumeEvent(buffer.trim())
  }
}

export function streamMessageNewSession({
  question,
  modelType,
  token,
  onSessionId,
  onChunk,
  onDone,
}) {
  return streamRequest({
    endpoint: '/AI/chat/send-stream-new-session',
    payload: { question, modelType },
    token,
    onSessionId,
    onChunk,
    onDone,
  })
}

export function streamMessage({
  sessionId,
  question,
  modelType,
  token,
  onChunk,
  onDone,
}) {
  return streamRequest({
    endpoint: '/AI/chat/send-stream',
    payload: { sessionId, question, modelType },
    token,
    onChunk,
    onDone,
  })
}
