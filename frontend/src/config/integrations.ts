import type { DeploymentCapabilityKey } from './deploymentCapabilities'

export const CHROME_EXTENSION_URL =
  'https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd?utm_source=item-share-cb'

export const CLAWHUB_SKILL_URL = 'https://clawhub.ai/lyingbug/weknora'

export type IntegrationTab = 'im' | 'embed' | 'api' | 'chrome' | 'claw'

// v2.3: Claw Skill 入口隐藏（仅前端隐藏，后端功能保留）。
// 从 INTEGRATION_TABS / INTEGRATION_PREVIEW_ITEMS 移除后：
// - 设置导航与侧边栏悬停预览不再出现该页签；
// - 旧书签 tab=claw 会回退到 integration-im；
// - 深链接 ?section=integration-claw 由 Settings.vue 的
//   HIDDEN_SETTING_SECTIONS 守卫拒绝（role-denied 兜底 + navItems 回退）。
export const INTEGRATION_TABS: IntegrationTab[] = ['im', 'embed', 'api', 'chrome']

/** Aligns with Settings.vue SECTION_MIN_ROLE.api and router.go g.Owner() on /api-principal-config. */
export type IntegrationTabRole = 'viewer' | 'contributor' | 'admin' | 'owner'

export const INTEGRATION_TAB_MIN_ROLE: Partial<Record<IntegrationTab, IntegrationTabRole>> = {
  api: 'owner',
}

export const INTEGRATION_TAB_CAPABILITY: Partial<Record<IntegrationTab, DeploymentCapabilityKey>> = {
  im: 'integrations.im',
  embed: 'integrations.embed',
  api: 'integrations.api',
}

export type IntegrationPreviewIcon =
  | { type: 'icon'; name: string }
  | { type: 'emoji'; value: string }

/** Sidebar hover preview + Integrations modal nav — add new entries here. */
export const INTEGRATION_PREVIEW_ITEMS: Array<{
  key: IntegrationTab
  icon: IntegrationPreviewIcon
}> = [
  { key: 'im', icon: { type: 'icon', name: 'chat-message' } },
  { key: 'embed', icon: { type: 'icon', name: 'code' } },
  { key: 'api', icon: { type: 'icon', name: 'secured' } },
  { key: 'chrome', icon: { type: 'icon', name: 'extension' } },
]
