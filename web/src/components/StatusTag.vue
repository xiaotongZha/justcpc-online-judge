<script setup lang="ts">
import { computed } from 'vue'
import { statusType } from '../store'

const props = defineProps<{ status: string }>()
const type = computed(() => (statusType[props.status] ?? 'info') as any)
const label = computed(() => (props.status === 'waiting' ? '等待评测' : props.status))
</script>

<template>
  <el-tag :type="type" size="small" effect="light" class="status-tag">{{ label }}</el-tag>
</template>
