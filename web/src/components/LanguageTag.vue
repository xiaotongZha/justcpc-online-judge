<script setup lang="ts">
import { computed } from 'vue'
import { languageLabel } from '../store'

const props = defineProps<{ language: string }>()
const label = computed(() => languageLabel[props.language] ?? props.language)
</script>

<template>
  <el-tag size="small" type="info" effect="plain">{{ label }}</el-tag>
</template>
