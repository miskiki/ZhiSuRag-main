<template>
  <div class="account-management">
    <header class="section-header">
      <h2>{{ t('accountManagement.title') }}</h2>
      <p class="section-description">{{ t('accountManagement.description') }}</p>
    </header>

    <div class="toolbar">
      <t-input
        v-model="search"
        class="toolbar__search"
        clearable
        :placeholder="t('accountManagement.searchPlaceholder')"
        @enter="onSearch"
        @clear="onSearch"
      >
        <template #prefix-icon><t-icon name="search" /></template>
      </t-input>
      <t-select
        v-model="statusFilter"
        class="toolbar__filter"
        clearable
        :placeholder="t('accountManagement.statusFilterPlaceholder')"
        @change="onSearch"
      >
        <t-option :label="t('accountManagement.statusActive')" value="active" />
        <t-option :label="t('accountManagement.statusDisabled')" value="disabled" />
      </t-select>
      <t-button variant="outline" @click="onSearch">
        <template #icon><t-icon name="refresh" /></template>
        {{ t('accountManagement.refresh') }}
      </t-button>
      <t-button theme="primary" @click="openCreate">
        <template #icon><t-icon name="add" /></template>
        {{ t('accountManagement.create') }}
      </t-button>
    </div>

    <div v-if="loading" class="state">
      <t-loading size="small" />
      <span>{{ t('accountManagement.loading') }}</span>
    </div>
    <div v-else-if="users.length === 0" class="state state--empty">
      <t-empty :description="t('accountManagement.empty')" />
    </div>
    <div v-else class="table-wrap">
      <table class="user-table">
        <thead>
          <tr>
            <th>{{ t('accountManagement.colUser') }}</th>
            <th>{{ t('accountManagement.colStatus') }}</th>
            <th>{{ t('accountManagement.colFlags') }}</th>
            <th>{{ t('accountManagement.colCreatedAt') }}</th>
            <th class="user-table__actions-heading">{{ t('accountManagement.colActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>
              <div class="user-cell">
                <span class="user-cell__name">{{ u.username }}</span>
                <span class="user-cell__email">{{ u.email }}</span>
              </div>
            </td>
            <td>
              <t-tag :theme="u.is_active ? 'success' : 'danger'" variant="light" size="small">
                {{ u.is_active ? t('accountManagement.statusActive') : t('accountManagement.statusDisabled') }}
              </t-tag>
            </td>
            <td>
              <t-tag v-if="u.is_system_admin" theme="warning" variant="light" size="small">
                {{ t('accountManagement.badgeSystemAdmin') }}
              </t-tag>
              <span v-else class="muted">—</span>
            </td>
            <td><span class="muted">{{ formatDate(u.created_at) }}</span></td>
            <td>
              <div class="user-table__actions">
                <t-button variant="text" size="small" @click="openDetail(u)">
                  {{ t('accountManagement.actions.detail') }}
                </t-button>
                <t-popconfirm
                  v-if="u.id !== currentUserId"
                  :content="u.is_active
                    ? t('accountManagement.disableConfirm', { name: u.username })
                    : t('accountManagement.enableConfirm', { name: u.username })"
                  :confirm-btn="{ content: t('accountManagement.actions.confirm'), theme: u.is_active ? 'danger' : 'primary' }"
                  :cancel-btn="{ content: t('accountManagement.actions.cancel') }"
                  placement="bottom-right"
                  @confirm="toggleStatus(u)"
                >
                  <t-button
                    variant="text"
                    size="small"
                    :theme="u.is_active ? 'danger' : 'primary'"
                    :loading="statusBusyId === u.id"
                  >
                    {{ u.is_active ? t('accountManagement.actions.disable') : t('accountManagement.actions.enable') }}
                  </t-button>
                </t-popconfirm>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="pagination-row">
        <t-pagination
          :current="page"
          :page-size="pageSize"
          :total="total"
          :show-page-size="false"
          @change="onPageChange"
        />
      </div>
    </div>

    <!-- 新建账号（复用既有 createSystemUser：自动生成密码一次性展示） -->
    <SettingDrawer
      :visible="createVisible"
      class="account-create-drawer"
      :title="t('accountManagement.create')"
      :description="t('accountManagement.createDescription')"
      icon="usergroup"
      width="480px"
      :min-width="420"
      :max-width="720"
      storage-key="setting-drawer:width:account-create"
      :close-on-overlay-click="false"
      :confirm-text="t('accountManagement.create')"
      :confirm-loading="creating"
      @update:visible="createVisible = $event"
      @confirm="submitCreate"
    >
      <div class="create-form">
        <div class="create-form__row">
          <label>{{ t('accountManagement.formUsername') }}</label>
          <t-input v-model="createForm.username" :placeholder="t('accountManagement.formUsernamePlaceholder')" />
        </div>
        <div class="create-form__row">
          <label>{{ t('accountManagement.formEmail') }}</label>
          <t-input v-model="createForm.email" :placeholder="t('accountManagement.formEmailPlaceholder')" />
        </div>
        <t-alert theme="info" :message="t('accountManagement.createPasswordHint')" class="create-form__hint" />
        <t-alert
          v-if="generatedPassword"
          theme="warning"
          :message="t('accountManagement.generatedPassword', { password: generatedPassword })"
          class="create-form__hint"
        />
      </div>
    </SettingDrawer>

    <AccountDetailDrawer
      v-model:visible="detailVisible"
      :user-id="detailUserId"
      @changed="reload"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'
import AccountDetailDrawer from './AccountDetailDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import {
  adminListUsers,
  adminSetUserStatus,
  createSystemUser,
  type AdminUserListItem,
} from '@/api/system'

const { t } = useI18n()
const authStore = useAuthStore()
const currentUserId = computed(() => authStore.currentUserId)

const loading = ref(false)
const users = ref<AdminUserListItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const search = ref('')
const statusFilter = ref<'active' | 'disabled' | ''>('')

const createVisible = ref(false)
const creating = ref(false)
const createForm = ref({ username: '', email: '' })
const generatedPassword = ref('')

const detailVisible = ref(false)
const detailUserId = ref('')

const statusBusyId = ref('')

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

async function reload() {
  loading.value = true
  try {
    const resp = await adminListUsers({
      search: search.value.trim() || undefined,
      page: page.value,
      page_size: pageSize,
      is_active: statusFilter.value === 'active' ? true : statusFilter.value === 'disabled' ? false : undefined,
    })
    users.value = resp.users ?? []
    total.value = resp.total ?? 0
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.loadFailed'))
  } finally {
    loading.value = false
  }
}

function onSearch() {
  page.value = 1
  void reload()
}

function onPageChange(next: number) {
  page.value = next
  void reload()
}

function openCreate() {
  createForm.value = { username: '', email: '' }
  generatedPassword.value = ''
  createVisible.value = true
}

async function submitCreate() {
  const username = createForm.value.username.trim()
  const email = createForm.value.email.trim()
  if (!username || !email) {
    MessagePlugin.warning(t('accountManagement.toasts.formIncomplete'))
    return
  }
  creating.value = true
  try {
    const resp = await createSystemUser({ username, email })
    generatedPassword.value = resp.generated_password ?? ''
    if (generatedPassword.value) {
      // 自动生成密码仅返回一次：保持抽屉打开展示，不自动关闭。
      MessagePlugin.success(t('accountManagement.toasts.created'))
    } else {
      createVisible.value = false
      MessagePlugin.success(t('accountManagement.toasts.created'))
    }
    await reload()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.createFailed'))
  } finally {
    creating.value = false
  }
}

function openDetail(u: AdminUserListItem) {
  detailUserId.value = u.id
  detailVisible.value = true
}

async function toggleStatus(u: AdminUserListItem) {
  statusBusyId.value = u.id
  try {
    await adminSetUserStatus(u.id, !u.is_active)
    MessagePlugin.success(t('accountManagement.toasts.statusChanged'))
    await reload()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('accountManagement.toasts.statusFailed'))
  } finally {
    statusBusyId.value = ''
  }
}

onMounted(reload)
</script>

<style lang="less" scoped>
.account-management {
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

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;

  &__search {
    width: 280px;
  }

  &__filter {
    width: 160px;
  }
}

.state {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: center;
  padding: 32px 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;

  &--empty {
    display: block;
  }
}

.table-wrap {
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  overflow: hidden;
}

.user-table {
  width: 100%;
  border-collapse: collapse;

  th,
  td {
    padding: 10px 14px;
    text-align: left;
    font-size: 13px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  thead th {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);
    font-weight: 500;
  }

  tbody tr:last-child td {
    border-bottom: none;
  }

  tbody tr:hover td {
    background: var(--td-bg-color-container-hover);
  }
}

.user-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;

  &__name {
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__email {
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }
}

.muted {
  color: var(--td-text-color-secondary);
}

.user-table__actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  padding: 12px 14px;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 14px;

  &__row {
    display: flex;
    flex-direction: column;
    gap: 6px;

    label {
      font-size: 13px;
      color: var(--td-text-color-primary);
      font-weight: 500;
    }
  }

  &__hint {
    margin-top: 4px;
  }
}
</style>
