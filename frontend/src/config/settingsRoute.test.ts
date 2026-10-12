import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildSettingsRouteQuery,
  integrationSectionKey,
  isIntegrationSection,
  normalizeSettingsSection,
  settingsQueryUnchanged,
} from './settingsRoute'

test('every settings nav item writes only section', () => {
  assert.deepEqual(
    buildSettingsRouteQuery('models', {
      section: 'system-global',
      tab: 'im',
      agentId: 'agt_1',
    }),
    { section: 'models' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('general', { section: 'system-global' }),
    { section: 'general' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('runtime-queues', { section: 'system-global' }),
    { section: 'runtime-queues' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery(integrationSectionKey('claw'), {
      section: 'integrations',
      tab: 'im',
    }),
    { section: 'integration-claw' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery(integrationSectionKey('api'), {
      section: 'integrations',
      tab: 'im',
      agentId: 'agt_1',
    }),
    { section: 'integration-api', agentId: 'agt_1' },
  )
})

test('legacy api / integrations / bare-tab query strings normalize to nav keys', () => {
  assert.equal(normalizeSettingsSection('api'), 'integration-api')
  // v2.3: Claw Skill 入口隐藏——claw 不再是合法页签。
  // 旧书签 tab=claw 走 integrationTabFromSection 回退到 integration-im；
  // section=integration-claw 原样透传，由 Settings.vue 的
  // HIDDEN_SETTING_SECTIONS 守卫拒绝（role-denied 兜底 + navItems 回退）。
  assert.equal(normalizeSettingsSection('claw'), 'claw')
  assert.equal(normalizeSettingsSection('integration-claw'), 'integration-claw')
  assert.equal(normalizeSettingsSection('integrations', 'claw'), 'integration-im')
  assert.equal(normalizeSettingsSection('integrations', 'embed'), 'integration-embed')
  assert.equal(normalizeSettingsSection('integrations'), 'integration-im')
  assert.equal(normalizeSettingsSection('integration-chrome'), 'integration-chrome')
  assert.equal(normalizeSettingsSection('system-global'), 'system-global')
  assert.equal(isIntegrationSection('integration-chrome'), true)
  assert.equal(isIntegrationSection('models'), false)
  assert.equal(isIntegrationSection('integration-unknown'), false)
})

test('canonical settings query skips a redundant replace', () => {
  assert.equal(
    settingsQueryUnchanged(
      { section: 'integration-claw' },
      { section: 'integration-claw' },
    ),
    true,
  )
  assert.equal(
    settingsQueryUnchanged(
      { section: 'integrations', tab: 'claw' },
      { section: 'integration-claw' },
    ),
    false,
  )
})
