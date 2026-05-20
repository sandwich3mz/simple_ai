const MODEL_LABELS = {
  qwen: 'Qwen',
  deepseek: 'DeepSeek',
}
export function modelLabel(modelType) {
  return MODEL_LABELS[modelType] || modelType || 'LLM'
}

export function resolveBaseModelSwitch({ currentSessionId, messageCount, nextModelType }) {
  const hasActiveSession = Boolean(currentSessionId || messageCount > 0)
  return {
    shouldStartNewSession: hasActiveSession,
    alertText: hasActiveSession
      ? `已切换到底座模型 ${modelLabel(nextModelType)}，下一条消息将自动创建新会话。`
      : `已切换到底座模型 ${modelLabel(nextModelType)}。`,
  }
}
