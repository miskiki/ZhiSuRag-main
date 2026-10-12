<template>
  <SettingDrawer
    :visible="visible"
    class="account-detail-drawer"
    :title="detail?.user?.username || t('accountManagement.detail.title')"
    :description="detail?.user?.email || t('accountManagement.detail.description')"
    icon="usergroup"
    width="640px"
    :min-width="520"
    :max-width="960"
    storage-key="setting-drawer:width:account-detail"
    :close-on-overlay-click="!busy"
    @update:visible="$emit('update:visible', $event)"
  >
    <div v-if="loading" class="detail-state">
      <t-loading size="small" />
      <span>{{ t('accountManagement.loading') }}</span>
    </div>

    <template v-else-if="detail">
      <!-- 区块一：基本信息 -->
      <section class="detail-block">
        <h3 class="detail-block__title">{{ t('accountManagement.detail.profileTitle') }}</h3>
        <div class="profile-grid">
          <div class="profile-grid__item">
            <label>{{ t('accountManagement.colUserId') }}</label>
            <code class="profile-grid__code">{{ detail.user.id }}</code>
          </div>
          <div class="profile-grid__item">
            <label>{{ t('accountManagement.colCreatedAt') }}</label>
            <span>{{ formatDate(detail.user.created_at) }}</span>
          </div>
          <div class="profile-grid__item">
            <label>{{ t('accountManagement.colStatus') }}</label>
            <t-tag :theme="detail.user.is_active ? 'success' : 'danger'" variant="light" size="small">
              {{ detail.user.is_active ? t('accountManagement.statusActive') : t('accountManagement.statusDisabled') }}
            </t-tag>
          </div>
          <div class="profile-grid__item">
            <label>{{ t('accountManagement.colFlags') }}</label>
            <div class="flag-tags">
              <t-tag v-if="detail.user.is_system_admin" theme="warning" variant="light" size="small">
                {{ t('accountManagement.badgeSystemAdmin') }}
              </t-tag>
              <t-tag
                v-if="detail.user.can_access_all_tenants"
                theme="primary"
                variant="light"
                size="small"
              >
                {{ t('accountManagement.badgeSuperuser') }}
              </t-tag>
              <span v-if="!detail.user.is_system_admin && !detail.user.can_access_all_tenants" class="muted">—</span>
            </div>
          </div>
        </div>

        <div class="profile-edit" v-if="canEditProfile">
          <div class="profile-edit__row">
            <label>{{ t('accountManagement.formUsername') }}</label>
            <t-input v-model="editUsername" :placeholder="detail.user.username" :disabled="profileBusy" />
          </div>
          <div class="profile-edit__row">
            <label>{{ t('accountManagement.formEmail') }}</label>
            <t-popconfirm
              v-if="editEmail !== detail.user.email"
              v-model:visible="emailPopconfirmVisible"
              :content="t('accountManagement.detail.emailChangeConfirm', { email: editEmail })"
              :confirm-btn="{ content: t('accountManagement.actions.confirm'), theme: 'primary' }"
              :cancel-btn="{ content: t('accountManagement.actions.cancel') }"
              placement="bottom-left"
            >
              <t-input
                v-model="editEmail"
                :placeholder="detail.user.email"
                :disabled="profileBusy"
                @enter="submitProfile"
              />
            </t-popconfirm>
            <t-input
              v-else
              v-model="editEmail"
              :placeholder="detail.user.email"
              :disabled="profileBusy"
              @enter="submitProfile"
            />
          </div>
          <div class="profile-edit__actions">
            <t-button
              size="small"
              variant="outline"
              :disabled="!profileDirty || profileBusy"
              :loading="profileBusy"
              @click="submitProfile"
            >
              {{ t('accountManagement.actions.save') }}
            </t-button>
            <t-button size="small" variant="text" :disabled="!profileDirty || profileBusy" @click="resetProfileEdit">
              {{ t('accountManagement.actions.cancel') }}
            </t-button>
          </div>
        </div>
      </section>

      <!-- 区块二：空间与角色 -->
      <section class="detail-block">
        <h3 class="detail-block__title">{{ t('accountManagement.detail.membershipsTitle') }}</h3>
        <div v-if="detail.memberships.length === 0" class="detail-block__empty">
          {{ t('accountManagement.detail.noMemberships') }}
        </div>
        <div
          v-for="m in detail.memberships"
          :key="m.tenant_id"
          class="membership-row"
        >
          <div class="membership-row__main">
            <div class="membership-row__name">
              <t-icon name="user-circle" size="15px" />
              <span>{{ m.tenant_name || t('accountManagement.detail.unknownTenant') }}</span>
              <t-tag :theme="membershipStatusTheme(m.status)" variant="light" size="small">
                {{ t(`accountManagement.membershipStatus.${m.status}`) }}
              </t-tag>
            </div>
            <div class="membership-row__meta">
              {{ t('accountManagement.detail.joinedAt') }} {{ formatDate(m.joined_at) }}
            </div>
          </div>
          <div class="membership-row__actions" v-if="canEditMemberships">
            <t-select
              :value="m.role"
              size="small"
              class="membership-row__role"
              :disabled="busy"
              @change="(role: AdminMembershipView['role']) => confirmRoleChange(m, role)"
            >
              <t-option v-for="r in EDITABLE_ROLES" :key="r" :label="t(`accountManagement.roles.${r}`)" :value="r" />
            </t-select>
            <t-popconfirm
              :content="t('accountManagement.detail.removeMembershipConfirm', { tenant: m.tenant_name || m.tenant_id })"
              :confirm-btn="{ content: t('accountManagement.actions.confirm'), theme: 'danger' }"
              :cancel-btn="{ content: t('accountManagement.actions.cancel') }"
              placement="bottom-right"
              @confirm="removeMembership(m)"
            >
              <t-button variant="text" theme="danger" size="small" :loading="busyTenantId === `${m.tenant_id}`">
                {{ t('accountManagement.detail.removeMembership') }}
              </t-button>
            </t-popconfirm>
          </div>
        </div>
      </section>

      <!-- 区块三：登录会话 -->
      <section class="detail-block">
        <div class="detail-block__title-row">
          <h3 class="detail-block__title">{{ t('accountManagement.detail.sessionsTitle') }}</h3>
          <t-popconfirm
            v-if="canForceLogout"
            :content="t('accountManagement.detail.forceLogoutConfirm')"
            :confirm-btn="{ content: t('accountManagement.actions.confirm'), theme: 'danger' }"
            :cancel-btn="{ content: t('accountManagement.actions.cancel') }"
            placement="bottom-right"
            @confirm="forceLogout"
          >
            <t-button variant="outline" theme="danger" size="small" :loading="busy">
              <template #icon><t-icon name="logout" /></template>
              {{ t('accountManagement.detail.forceLogout') }}
            </t-button>
          </t-popconfirm>
        </div>
        <div v-if="sessions.length === 0" class="detail-block__empty">
          {{ t('accountManagement.detail.noSessions') }}
        </div>
        <div
          v-for="s in sessions"
          :key="s.session_id || '__early__'"
          class="session-row"
        >
          <div class="session-row__main">
            <div class="session-row__device">
              <t-icon name="laptop" size="15px" />
              <span>{{ s.device_label || t('accountManagement.detail.unknownDevice') }}</span>
            </div>
            <div class="session-row__meta">
              <span v-if="s.device_ip"><t-icon name="location" size="12px" /> {{ s.device_ip }}</span>
              <span><t-icon name="time" size="12px" /> {{ t('accountManagement.detail.loginAt') }} {{ formatDate(s.created_at) }}</span>
              <span><t-icon name="refresh" size="12px" /> {{ t('accountManagement.detail.lastActiveAt') }} {{ formatDate(s.last_refresh_at) }}</span>
            </div>
          </div>
          <t-button
            v-if="canKickSessions && s.session_id"
            variant="text"
            theme="danger"
            size="small"
            :loading="kickingSessionId === s.session_id"
            @click="kickSession(s)"
          >
            {{ t('accountManagement.detail.kickSession') }}
          </t-button>
        </div>
      </section>
    </template>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import {
  adminGetUserDetail,
  adminListUserSessions,
  adminRevokeUserSession,
  adminForceLogout,
  adminRemoveMembership,
  adminUpdateMembershipRole,
  adminUpdateUser,
  type AdminMembershipView,
  type AdminUserDetail,
  type AdminUserLoginSession,
} from '@/api/system'

const props = defineProps<{
  visible: boolean
  userId: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'changed'): void
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const currentUserId = computed(() => authStore.currentUserId)

// 后端自我保护：target==caller 的写操作一律 400。前端对齐不渲染危险入口，
// 而是引导操作者去个人面板（资料/登录设备/空间成员）。
const isSelf = computed(() => !!props.userId && props.userId === currentUserId.value)
const canEditProfile = computed(() => !isSelf.value)
const canEditMemberships = computed(() => !isSelf.value)
const canForceLogout = computed(() => !isSelf.value)
// 踢单会话同样受 target==caller 守卫约束（会话治理对自己也走个人面板）。
const canKickSessions = computed(() => !isSelf.value)

// 角色可选项：owner 也可以设置（最后 Owner 保护由后端 ErrLastOwner 兜底）。
const EDITABLE_ROLES: AdminMembershipView['role'][] = ['owner', 'admin', 'contributor', 'viewer']

const loading = ref(false)
const busy = ref(false)
const busyTenantId = ref('')
const detail = ref<AdminUserDetail | null>(null)
const sessions = ref<AdminUserLoginSession[]>([])
const kickingSessionId = ref('')

const editUsername = ref('')
const editEmail = ref('')
const profileBusy = ref(false)
const emailPopconfirmVisible = ref(false)

const profileDirty = computed(
  () =>
    !!detail.value &&
    (editUsername.value !== detail.value.user.username || editEmail.value !== detail.value.user.email),
)

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function membershipStatusTheme(status: AdminMembershipView['status']) {
  switch (status) {
    case 'active':
      return 'success'
    case 'invited':
      return 'warning'
    default:
      return 'default'
  }
}

function resetProfileEdit() {
  editUsername.value = detail.value?.user.username ?? ''
  editEmail.value = detail.value?.user.email ?? ''
}

async function loadDetail() {
  if (!props.userId) return
  loading.value = true
  try {
    detail.value = await adminGetUserDetail(props.userId)
    sessions.value = await adminListUserSessions(props.userId)
    resetProfileEdit()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.loadFailed'))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.visible, props.userId] as const,
  ([visible]) => {
    if (visible && props.userId) {
      detail.value = null
      sessions.value = []
      void loadDetail()
    }
  },
  { immediate: true },
)

async function submitProfile() {
  if (!detail.value || !profileDirty.value) return
  const patch: { username?: string; email?: string } = {}
  if (editUsername.value !== detail.value.user.username) patch.username = editUsername.value.trim()
  if (editEmail.value !== detail.value.user.email) patch.email = editEmail.value.trim()
  if (!patch.username && !patch.email) return
  if (patch.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(patch.email)) {
    MessagePlugin.warning(t('accountManagement.toasts.emailInvalid'))
    return
  }
  profileBusy.value = true
  try {
    if (patch.email && patch.email !== detail.value.user.email) {
      // 邮箱变更属敏感操作：popconfirm 已在输入框上挂载，这里兜底再确认一次语义。
      const ok = emailPopconfirmVisible.value ? true : window.confirm(t('accountManagement.detail.emailChangeConfirm', { email: patch.email }))
      if (!ok) return
    }
    const updated = await adminUpdateUser(props.userId, patch)
    if (detail.value) detail.value.user = { ...detail.value.user, ...updated }
    resetProfileEdit()
    MessagePlugin.success(t('accountManagement.toasts.profileSaved'))
    emit('changed')
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.profileFailed'))
  } finally {
    profileBusy.value = false
  }
}

async function confirmRoleChange(m: AdminMembershipView, role: AdminMembershipView['role']) {
  if (role === m.role) return
  busy.value = true
  busyTenantId.value = `${m.tenant_id}`
  try {
    await adminUpdateMembershipRole(props.userId, m.tenant_id, role)
    MessagePlugin.success(
      t('accountManagement.toasts.roleChanged', {
        tenant: m.tenant_name || m.tenant_id,
        role: t(`accountManagement.roles.${role}`),
      }),
    )
    await loadDetail()
    emit('changed')
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.roleFailed'))
    await loadDetail()
  } finally {
    busy.value = false
    busyTenantId.value = ''
  }
}

async function removeMembership(m: AdminMembershipView) {
  busy.value = true
  busyTenantId.value = `${m.tenant_id}`
  try {
    await adminRemoveMembership(props.userId, m.tenant_id)
    MessagePlugin.success(t('accountManagement.toasts.membershipRemoved'))
    await loadDetail()
    emit('changed')
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.membershipFailed'))
  } finally {
    busy.value = false
    busyTenantId.value = ''
  }
}

async function forceLogout() {
  busy.value = true
  try {
    await adminForceLogout(props.userId)
    MessagePlugin.success(t('accountManagement.toasts.forceLoggedOut'))
    await loadDetail()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.forceLogoutFailed'))
  } finally {
    busy.value = false
  }
}

async function kickSession(s: AdminUserLoginSession) {
  if (!s.session_id) return
  kickingSessionId.value = s.session_id
  try {
    await adminRevokeUserSession(props.userId, s.session_id)
    MessagePlugin.success(t('accountManagement.toasts.sessionKicked'))
    await loadDetail()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.kickFailed'))
  } finally {
    kickingSessionId.value = ''
  }
}
</script>

<style lang="less" scoped>
.account-detail-drawer {
  --detail-gap: 18px;
}

.detail-state {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: center;
  padding: 32px 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.detail-block {
  margin-bottom: var(--detail-gap);
  padding: 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;

  &__title {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  &__title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;

    .detail-block__title {
      margin-bottom: 0;
    }
  }

  &__empty {
    padding: 12px 0;
    font-size: 13px;
    color: var(--td-text-color-secondary);
  }
}

.profile-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 16px;
  margin-bottom: 14px;

  &__item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;

    label {
      color: var(--td-text-color-secondary);
      font-size: 12px;
    }
  }

  &__code {
    font-size: 12px;
    color: var(--td-text-color-primary);
    word-break: break-all;
  }
}

.flag-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.profile-edit {
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-top: 1px dashed var(--td-component-stroke);
  padding-top: 12px;

  &__row {
    display: flex;
    flex-direction: column;
    gap: 4px;

    label {
      font-size: 12px;
      color: var(--td-text-color-secondary);
    }
  }

  &__actions {
    display: flex;
    gap: 8px;
  }
}

.membership-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child {
    border-bottom: none;
  }

  &__name {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--td-text-color-primary);
    font-weight: 500;
  }

  &__meta {
    margin-top: 4px;
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }

  &__actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  &__role {
    width: 140px;
  }
}

.session-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child {
    border-bottom: none;
  }

  &__device {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--td-text-color-primary);
    font-weight: 500;
  }

  &__meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    margin-top: 4px;
    font-size: 12px;
    color: var(--td-text-color-secondary);

    span {
      display: inline-flex;
      align-items: center;
      gap: 3px;
    }
  }
}

.muted {
  color: var(--td-text-color-secondary);
}
</style>
