<script setup lang="ts">
import { computed } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import markedKatex from 'marked-katex-extension'
import 'katex/dist/katex.min.css'

marked.use(markedKatex({ throwOnError: false, nonStandard: true }))

const props = defineProps<{ markdown: string }>()
const html = computed(() => DOMPurify.sanitize(marked.parse(props.markdown || '') as string))
</script>

<template>
  <article class="statement" v-html="html" />
</template>
