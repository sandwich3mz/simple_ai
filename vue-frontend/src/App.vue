<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { login, register, sendCaptcha } from './api/auth'
import {
  getHistory,
  getSessions,
  sendMessage,
  sendMessageNewSession,
  streamMessage,
  streamMessageNewSession,
} from './api/chat'
import { normalizeError, setAuthToken } from './api/client'

const TOKEN_KEY = 'simple_ai_token'
const MODEL_KEY = 'simple_ai_model'

const token = ref(localStorage.getItem(TOKEN_KEY) || '')
const modelType = ref(localStorage.getItem(MODEL_KEY) || 'qwen')
const streamMode = ref(true)

const sessions = ref([])
const currentSessionId = ref('')
const messages = ref([])
const inputText = ref('')

const activeAuthTab = ref('login')
const loginUsername = ref('')
const loginPassword = ref('')
const registerEmail = ref('')
const registerCaptcha = ref('')
const registerPassword = ref('')

const loadingSessions = ref(false)
const loadingHistory = ref(false)
const sending = ref(false)
const authLoading = ref(false)
const captchaLoading = ref(false)

const alertText = ref('')
const alertType = ref('info')
const messageContainer = ref(null)

setAuthToken(token.value)

const canSend = computed(() => token.value && inputText.value.trim() && !sending.value)
const currentSessionTitle = computed(() => {
  const match = sessions.value.find((item) => item.sessionId === currentSessionId.value)
  return match?.name || '新会话'
})

watch(modelType, (value) => {
  localStorage.setItem(MODEL_KEY, value)
})

watch(
  () => messages.value.length,
  () => scrollToBottom(),
)

function showAlert(message, type = 'error') {
  alertText.value = message
  alertType.value = type
}

function clearAlert() {
  alertText.value = ''
}

function setToken(newToken) {
  token.value = newToken || ''
  setAuthToken(token.value)
  if (token.value) {
    localStorage.setItem(TOKEN_KEY, token.value)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (!messageContainer.value) {
      return
    }
    messageContainer.value.scrollTop = messageContainer.value.scrollHeight
  })
}

function sessionLabel(session) {
  if (!session?.name || session.name === session.sessionId) {
    return `会话 ${session.sessionId?.slice(0, 8)}`
  }
  return session.name
}

function bindSession(sessionId, titleHint) {
  if (!sessionId) {
    return
  }
  currentSessionId.value = sessionId
  const exists = sessions.value.some((item) => item.sessionId === sessionId)
  if (!exists) {
    sessions.value.unshift({
      sessionId,
      name: titleHint || sessionId,
    })
  }
}

function normalizeHistory(history) {
  return (history || []).map((item) => ({
    role: item.is_user ? 'user' : 'assistant',
    content: item.content || '',
  }))
}

async function refreshSessions({ keepCurrent = true } = {}) {
  if (!token.value) {
    sessions.value = []
    currentSessionId.value = ''
    return
  }
  loadingSessions.value = true
  try {
    const data = await getSessions()
    sessions.value = data.sessions || []
    if (!keepCurrent || !sessions.value.some((item) => item.sessionId === currentSessionId.value)) {
      currentSessionId.value = sessions.value[0]?.sessionId || ''
    }
  } catch (error) {
    showAlert(normalizeError(error))
  } finally {
    loadingSessions.value = false
  }
}

async function loadHistory(sessionId) {
  if (!sessionId) {
    messages.value = []
    return
  }
  loadingHistory.value = true
  clearAlert()
  try {
    const data = await getHistory(sessionId)
    messages.value = normalizeHistory(data.history)
  } catch (error) {
    showAlert(normalizeError(error))
    messages.value = []
  } finally {
    loadingHistory.value = false
  }
}

async function selectSession(sessionId) {
  if (!sessionId || sessionId === currentSessionId.value) {
    return
  }
  currentSessionId.value = sessionId
  await loadHistory(sessionId)
}

function resetAuthForms() {
  loginUsername.value = ''
  loginPassword.value = ''
  registerEmail.value = ''
  registerCaptcha.value = ''
  registerPassword.value = ''
}

async function onLogin() {
  clearAlert()
  authLoading.value = true
  try {
    const data = await login(loginUsername.value.trim(), loginPassword.value)
    setToken(data.token || '')
    resetAuthForms()
    showAlert('登录成功', 'success')
    await refreshSessions()
    if (currentSessionId.value) {
      await loadHistory(currentSessionId.value)
    } else {
      messages.value = []
    }
  } catch (error) {
    showAlert(normalizeError(error))
  } finally {
    authLoading.value = false
  }
}

async function onRegister() {
  clearAlert()
  authLoading.value = true
  try {
    const data = await register(
      registerEmail.value.trim(),
      registerPassword.value,
      registerCaptcha.value.trim(),
    )
    setToken(data.token || '')
    resetAuthForms()
    showAlert('注册成功，已自动登录', 'success')
    await refreshSessions()
    messages.value = []
  } catch (error) {
    showAlert(normalizeError(error))
  } finally {
    authLoading.value = false
  }
}

async function onSendCaptcha() {
  clearAlert()
  captchaLoading.value = true
  try {
    await sendCaptcha(registerEmail.value.trim())
    showAlert('验证码已发送，请检查邮箱', 'success')
  } catch (error) {
    showAlert(normalizeError(error))
  } finally {
    captchaLoading.value = false
  }
}

function onLogout() {
  setToken('')
  sessions.value = []
  messages.value = []
  currentSessionId.value = ''
  showAlert('已退出登录', 'info')
}

async function sendPlain(question, assistantMessage) {
  if (!currentSessionId.value) {
    const data = await sendMessageNewSession(question, modelType.value)
    assistantMessage.content = data.Information || ''
    bindSession(data.sessionId, question)
    return
  }
  const data = await sendMessage(currentSessionId.value, question, modelType.value)
  assistantMessage.content = data.Information || ''
}

async function sendStream(question, assistantMessage) {
  if (!token.value) {
    throw new Error('请先登录')
  }
  if (!currentSessionId.value) {
    await streamMessageNewSession({
      question,
      modelType: modelType.value,
      token: token.value,
      onSessionId: (sessionId) => bindSession(sessionId, question),
      onChunk: (chunk) => {
        assistantMessage.content += chunk
        scrollToBottom()
      },
    })
    return
  }

  await streamMessage({
    sessionId: currentSessionId.value,
    question,
    modelType: modelType.value,
    token: token.value,
    onChunk: (chunk) => {
      assistantMessage.content += chunk
      scrollToBottom()
    },
  })
}

async function onSendMessage() {
  const question = inputText.value.trim()
  if (!question || !token.value || sending.value) {
    return
  }

  clearAlert()
  inputText.value = ''
  sending.value = true

  const userMessage = { role: 'user', content: question }
  const assistantMessage = { role: 'assistant', content: '' }

  messages.value.push(userMessage)
  messages.value.push(assistantMessage)
  scrollToBottom()

  try {
    if (streamMode.value) {
      await sendStream(question, assistantMessage)
    } else {
      await sendPlain(question, assistantMessage)
    }
    if (!assistantMessage.content.trim()) {
      assistantMessage.content = '(模型没有返回内容)'
    }
    await refreshSessions()
  } catch (error) {
    if (!assistantMessage.content) {
      assistantMessage.content = '请求失败，请重试。'
    }
    showAlert(normalizeError(error))
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  if (!token.value) {
    return
  }
  await refreshSessions()
  if (currentSessionId.value) {
    await loadHistory(currentSessionId.value)
  }
})
</script>

<template>
  <div class="app-shell">
    <div class="ambient"></div>
    <aside class="left-panel">
      <div class="brand">
        <h1>Simple AI</h1>
        <p>Vue3 Frontend for your Go backend</p>
      </div>

      <div class="panel auth-panel" v-if="!token">
        <div class="tabs">
          <button
            class="tab-btn"
            :class="{ active: activeAuthTab === 'login' }"
            @click="activeAuthTab = 'login'"
          >
            登录
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeAuthTab === 'register' }"
            @click="activeAuthTab = 'register'"
          >
            注册
          </button>
        </div>

        <div v-if="activeAuthTab === 'login'" class="auth-body">
          <label>用户名</label>
          <input v-model="loginUsername" placeholder="输入用户名" />
          <label>密码</label>
          <input v-model="loginPassword" type="password" placeholder="输入密码" />
          <button class="action-btn" :disabled="authLoading" @click="onLogin">
            {{ authLoading ? '登录中...' : '登录' }}
          </button>
        </div>

        <div v-else class="auth-body">
          <label>邮箱</label>
          <input v-model="registerEmail" placeholder="输入邮箱" />
          <label>验证码</label>
          <div class="inline">
            <input v-model="registerCaptcha" placeholder="输入验证码" />
            <button class="ghost-btn" :disabled="captchaLoading" @click="onSendCaptcha">
              {{ captchaLoading ? '发送中' : '发验证码' }}
            </button>
          </div>
          <label>密码</label>
          <input v-model="registerPassword" type="password" placeholder="设置密码" />
          <button class="action-btn" :disabled="authLoading" @click="onRegister">
            {{ authLoading ? '注册中...' : '注册并登录' }}
          </button>
        </div>
      </div>

      <div v-else class="panel session-panel">
        <div class="panel-head">
          <strong>会话列表</strong>
          <button class="ghost-btn" :disabled="loadingSessions" @click="refreshSessions()">
            刷新
          </button>
        </div>

        <div class="session-list">
          <button
            v-for="session in sessions"
            :key="session.sessionId"
            class="session-item"
            :class="{ active: session.sessionId === currentSessionId }"
            @click="selectSession(session.sessionId)"
          >
            {{ sessionLabel(session) }}
          </button>
          <div class="empty" v-if="!sessions.length">暂无会话，先发一条消息开始对话</div>
        </div>

        <button class="action-btn danger" @click="onLogout">退出登录</button>
      </div>
    </aside>

    <main class="chat-panel">
      <header class="chat-toolbar">
        <div>
          <small>当前会话</small>
          <h2>{{ token ? currentSessionTitle : '请先登录' }}</h2>
        </div>

        <div class="toolbar-actions">
          <select v-model="modelType">
            <option value="qwen">qwen</option>
            <option value="deepseek">deepseek</option>
          </select>
          <label class="stream-toggle">
            <input v-model="streamMode" type="checkbox" />
            流式输出
          </label>
        </div>
      </header>

      <div class="alert" :class="alertType" v-if="alertText">{{ alertText }}</div>

      <section class="message-list" ref="messageContainer">
        <div class="empty-message" v-if="!messages.length && !loadingHistory">
          登录后即可开始聊天，首次发送会自动创建会话。
        </div>
        <div
          v-for="(message, index) in messages"
          :key="`${message.role}-${index}`"
          class="message-row"
          :class="message.role"
        >
          <div class="bubble">{{ message.content }}</div>
        </div>
      </section>

      <footer class="composer">
        <textarea
          v-model="inputText"
          :disabled="!token || sending"
          placeholder="输入问题后按发送（Shift+Enter 换行）"
          @keydown.enter.exact.prevent="onSendMessage"
        ></textarea>
        <button class="action-btn" :disabled="!canSend" @click="onSendMessage">
          {{ sending ? '发送中...' : '发送' }}
        </button>
      </footer>
    </main>
  </div>
</template>
