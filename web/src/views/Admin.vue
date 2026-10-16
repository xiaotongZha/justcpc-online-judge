<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, refresh, type QueueStatus } from '../store'

const queue = ref<QueueStatus[]>([])
const loading = ref(false)
const retrying = ref(false)
const queueLabels: Record<string, string> = { queued: '排队中', running: '运行中', failed: '失败' }

async function loadQueue(manual = false) {
  if (manual) loading.value = true
  try {
    queue.value = (await api<QueueStatus[]>('/admin/judge-queue')) ?? []
  } catch {
    queue.value = []
  } finally {
    if (manual) loading.value = false
  }
}

async function retry() {
  retrying.value = true
  try {
    const result = await api<{ retried: number }>('/admin/judge-queue/retry', { method: 'POST' })
    ElMessage.success(`已重新入队 ${result.retried} 个失败任务`)
    await Promise.all([loadQueue(), refresh()])
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    retrying.value = false
  }
}

const counts = computed(() => {
  const c = { queued: 0, running: 0, failed: 0 }
  for (const item of queue.value) c[item.state]++
  return c
})

let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  loadQueue()
  timer = setInterval(loadQueue, 4000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>判题队列</h2>
        <p>查看任务执行状态与失败记录</p>
      </div>
      <el-button :loading="loading" @click="loadQueue(true)"
        ><el-icon><Refresh /></el-icon>刷新队列</el-button
      >
      <el-button type="primary" :disabled="counts.failed === 0" :loading="retrying" @click="retry"
        >重试失败任务</el-button
      >
    </div>
    <div class="oj-card">
      <div class="oj-card-header">
        <h3>任务列表</h3>
        <el-tag type="info" size="small">排队 {{ counts.queued }}</el-tag
        ><el-tag type="primary" size="small">运行中 {{ counts.running }}</el-tag
        ><el-tag type="danger" size="small">失败 {{ counts.failed }}</el-tag>
      </div>
      <div class="oj-table-wrap">
        <el-table :data="queue" v-loading="loading" empty-text="当前没有判题任务">
          <el-table-column prop="id" label="任务 ID" width="100" />
          <el-table-column prop="submission_id" label="提交 ID" width="110" />
          <el-table-column label="状态" width="130"
            ><template #default="{ row }"
              ><el-tag
                :type="
                  row.state === 'failed' ? 'danger' : row.state === 'running' ? 'primary' : 'info'
                "
                size="small"
                >{{ queueLabels[row.state] }}</el-tag
              ></template
            ></el-table-column
          >
          <el-table-column prop="attempt" label="尝试次数" width="110" align="right" />
          <el-table-column prop="error" label="错误信息" min-width="260" show-overflow-tooltip />
        </el-table>
      </div>
      <div class="oj-card-footer">失败任务保留在队列中，可重新加入评测；状态自动刷新。</div>
    </div>
  </div>
</template>
