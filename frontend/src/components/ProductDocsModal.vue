<template>
  <t-dialog v-model:visible="visible" :header="$t('general.helpAndDocs')" width="880px" placement="center"
    :footer="false" class="product-docs-dialog" @close="handleClose">
    <div class="product-docs-md" v-html="renderedHtml"></div>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import manualZh from '@/docs/product-manual.zh-CN.md?raw'
import manualEn from '@/docs/product-manual.en-US.md?raw'

/**
 * 产品说明书弹窗：左下角「帮助与文档」入口的落地页。
 * 内容以 Markdown 维护（src/docs/product-manual.*.md），打包期内联（?raw），
 * 无后端依赖；中文用户读中文版，其余语言回退英文版。
 */
const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()

const { locale } = useI18n()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const manualSource = computed(() =>
  locale.value.toLowerCase().startsWith('zh') ? manualZh : manualEn,
)

// 让说明书内的外链（如 GitHub 文档）在新标签页打开
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node instanceof Element && node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

const renderedHtml = computed(() =>
  DOMPurify.sanitize(marked.parse(manualSource.value, { gfm: true, async: false }) as string),
)

const handleClose = () => {
  emit('update:modelValue', false)
}
</script>

<!-- 弹窗被 Teleport 到 body 且内容由 v-html 注入（无 scope 属性），
     因此样式用非 scoped 的独立块，全部收敛在专属类名下。 -->
<style>
.product-docs-dialog .t-dialog__body {
  max-height: min(72vh, 760px);
  overflow-y: auto;
  padding-right: 8px;
}

.product-docs-md {
  color: var(--td-text-color-primary, #1f2329);
  font-size: 14px;
  line-height: 1.75;
}

.product-docs-md h1 {
  font-size: 22px;
  margin: 4px 0 16px;
  line-height: 1.4;
}

.product-docs-md h2 {
  font-size: 17px;
  margin: 24px 0 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
}

.product-docs-md h3 {
  font-size: 15px;
  margin: 18px 0 8px;
}

.product-docs-md p {
  margin: 8px 0;
}

.product-docs-md ul,
.product-docs-md ol {
  margin: 8px 0;
  padding-left: 22px;
}

.product-docs-md li {
  margin: 4px 0;
}

.product-docs-md table {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0;
  font-size: 13px;
}

.product-docs-md th,
.product-docs-md td {
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 7px 10px;
  text-align: left;
  word-break: break-word;
}

.product-docs-md th {
  background: var(--td-bg-color-secondarycontainer, #f3f3f3);
  font-weight: 600;
}

.product-docs-md blockquote {
  margin: 12px 0;
  padding: 8px 14px;
  border-left: 3px solid var(--td-brand-color, #0052d9);
  background: var(--td-bg-color-secondarycontainer, #f3f6fd);
  border-radius: 4px;
  color: var(--td-text-color-secondary, #4e5969);
}

.product-docs-md blockquote p {
  margin: 4px 0;
}

.product-docs-md code {
  background: var(--td-bg-color-secondarycontainer, #f3f3f3);
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 13px;
}

.product-docs-md a {
  color: var(--td-brand-color, #0052d9);
  text-decoration: none;
}

.product-docs-md a:hover {
  text-decoration: underline;
}

.product-docs-md hr {
  border: none;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
  margin: 18px 0;
}
</style>
