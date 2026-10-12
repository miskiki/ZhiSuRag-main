export type SettingsRoleKey = 'viewer' | 'contributor' | 'admin' | 'owner'

/**
 * Workspace-scoped settings access policy.
 *
 * Keep this as the single frontend source of truth for both the complete
 * Settings navigation and any shortcuts that lead into it. Backend route
 * guards remain authoritative.
 */
export const SETTINGS_SECTION_MIN_ROLE: Record<string, SettingsRoleKey> = {
  general: 'viewer',
  ollama: 'admin',
  weknoracloud: 'admin',
  models: 'viewer',
  websearch: 'admin',
  chathistory: 'admin',
  vectorstore: 'admin',
  parser: 'admin',
  storage: 'admin',
  sandbox: 'admin',
  mcp: 'admin',
  system: 'viewer',
  userprofile: 'viewer',
  tenant: 'viewer',
  members: 'viewer',
  mymemory: 'viewer',
  memory: 'admin',
  // v2.16 多端会话管理：个人资产（本人操作本人会话），全员可见自己的
  sessions: 'viewer',
}

/**
 * A management-labelled avatar shortcut has a stricter threshold than the
 * corresponding read-only Settings page.
 */
export const SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE = {
  members: 'owner',
  models: 'admin',
} as const satisfies Record<string, SettingsRoleKey>

export const SYSTEM_ADMIN_SETTINGS_SECTIONS = new Set([
  'system-global',
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
  // v2.20 账号管理：平台级账号/会话治理（SystemAdmin 专属分区）
  'account-management',
])

/**
 * v2.9 平台统一配置收口：以下分区的**写权限**收归系统管理员
 * （后端对应路由组已同步 SystemAdmin-only，见 routes_infra.go 与
 * UpdateTenantKV 的平台键判定；retrieval-config 走 ⌘K 面板同规则）。
 *
 * 注意三个边界：
 * 1. 这里**不控制可见性**——分区入口的角色门槛仍按
 *    SETTINGS_SECTION_MIN_ROLE（可见性阶梯不变，空间管理员保留只读），
 *    组件内部用 authStore.isSystemAdmin 锁定写控件；
 * 2. 不要把这些 key 塞进 SYSTEM_ADMIN_SETTINGS_SECTIONS——那个集合
 *    挂着「关闭设置跳转知识库列表」等系统管理组专属行为；
 * 3. KB 级「选用」（KBModelConfig / KBVectorStore / KBStorage 选实例）
 *    不在此列，继续由空间角色操作。
 */
export const PLATFORM_MANAGED_SECTIONS = new Set([
  'models',
  'ollama',
  'websearch',
  'vectorstore',
  'parser',
  'storage',
  'sandbox',
  'mcp',
])
