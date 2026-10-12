<template>
  <div class="session-settings">
    <div class="section-header">
      <h2>{{ t('sessionSettings.title') }}</h2>
      <p class="section-description">{{ t('sessionSettings.description') }}</p>
    </div>

    <t-loading :loading="loading" size="small" class="session-list-loading">
      <div v-if="!loading && sessions.length === 0" class="session-empty">
        <t-empty :description="t('sessionSettings.empty')" />
      </div>
      <div v-else class="session-list">
        <div
          v-for="session in sessions"
          :key="session.session_id || '__early__'"
          class="session-item"
          :class="{ 'session-item--current': session.current }"
        >
          <div class="session-item__main">
            <div class="session-item__device">
              <t-icon name="laptop" size="16px" class="session-item__icon" />
              <span class="session-item__label">
                {{ session.device_label || t('sessionSettings.unknownDevice') }}
              </span>
              <t-tag v-if="session.current" theme="success" variant="light" size="small">
                {{ t('sessionSettings.currentDevice') }}
              </t-tag>
            </div>
            <div class="session-item__meta">
              <span v-if="session.device_ip" class="session-item__meta-item">
                <t-icon name="location" size="12px" /> IP {{ session.device_ip }}
              </span>
              <span class="session-item__meta-item">
                <t-icon name="time" size="12px" />
                {{ t('sessionSettings.loginAt') }} {{ formatTime(session.created_at) }}
              </span>
              <span class="session-item__meta-item">
                <t-icon name="refresh" size="12px" />
                {{ t('sessionSettings.lastActiveAt') }} {{ formatTime(session.last_refresh_at) }}
              </span>
              <span class="session-item__meta-item">
                <t-icon name="history" size="12px" />
                {{ t('sessionSettings.expiresAt') }} {{ formatTime(session.expires_at) }}
              </span>
            </div>
          </div>
          <div class="session-item__actions">
            <t-button
              v-if="!session.current && session.session_id"
              theme="danger"
              variant="outline"
              size="small"
              :loading="kickingId === session.session_id"
              @click="confirmKick(session)"
            >
              {{ t('sessionSettings.kick') }}
            </t-button>
          </div>
        </div>
      </div>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { listLoginSessions, revokeLoginSession, type LoginSession } from '@/api/auth/sessions'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()

// v2.16 多端会话管理面板：个人资产（本人操作本人会话），全员可见自己的。
// 会话数据由后端按 session_id 聚合；踢除即时生效（下一请求 401 → 被踢
// 设备走现有 401→跳登录链路）。
const canView = computed(() => !!authStore.user?.id)

const loading = ref(false)
const kickingId = ref('')
const sessions = ref<LoginSession[]>([])

const formatTime = (iso: string) => {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString()
}

const loadSessions = async () => {
  loading.value = true
  try {
    const res = await listLoginSessions()
    sessions.value = res?.data ?? []
  } catch (error) {
    console.error('Failed to load login sessions:', error)
  } finally {
    loading.value = false
  }
}

const confirmKick = (session: LoginSession) => {
  const label = session.device_label || t('sessionSettings.unknownDevice')
  // eslint-disable-next-line no-alert
  const ok = window.confirm(t('sessionSettings.kickConfirm', { device: label }))
  if (!ok) return
  kickSession(session)
}

const kickSession = async (session: LoginSession) => {
  if (!session.session_id) return
  kickingId.value = session.session_id
  try {
    await revokeLoginSession(session.session_id)
    MessagePlugin.success(t('sessionSettings.toasts.kicked'))
    await loadSessions()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('sessionSettings.toasts.kickFailed'))
  } finally {
    kickingId.value = ''
  }
}

onMounted(async () => {
  if (!canView.value) return
  await loadSessions()
})
</script>

<style lang="less" scoped>
.session-settings {
  width: 100%;
}

.section-header {
  margin-bottom: 20px;

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 6px 0;
  }
}

.section-description {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
  line-height: 1.5;
}

.session-list-loading {
  min-height: 120px;
}

.session-empty {
  padding: 32px 0;
}

.session-list {
  display: flex;
  flex-direction: column;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 12px;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child {
    border-bottom: none;
  }

  &--current {
    background: var(--td-brand-color-light);
    border-radius: 8px;
  }
}

.session-item__device {
  display: flex;
  align-items: center;
  gap: 8px;

  .session-item__icon {
    color: var(--td-text-color-secondary);
  }
}

.session-item__label {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.session-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  margin-top: 6px;
}

.session-item__meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.session-item__actions {
  flex-shrink: 0;
}
</style>
