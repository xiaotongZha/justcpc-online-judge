<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, refresh, type QueueStatus } from '../store'

const queue = ref<QueueStatus[]>([])
const loading = ref(false)
const retrying = ref(false)

async function loadQueue() {
  try {
    queue.value = (await api<QueueStatus[]>('/admin/judge-queue')) ?? []
  } catch {
    queue.value = []
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
      <h2>管理员</h2>
      <el-button :loading="loading" @click="loadQueue">刷新队列</el-button>
      <el-button type="warning" :loading="retrying" @click="retry">重试失败任务</el-button>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-header">
        <h3>判题队列</h3>
        <el-tag type="info" size="small" effect="light">排队 {{ counts.queued }}</el-tag>
        <el-tag type="primary" size="small" effect="light">运行中 {{ counts.running }}</el-tag>
        <el-tag type="danger" size="small" effect="light">失败 {{ counts.failed }}</el-tag>
      </div>
      <div class="oj-table-wrap">
        <el-table :data="queue" stripe size="small">
          <el-table-column prop="id" label="任务 ID" width="90" />
          <el-table-column prop="submission_id" label="提交 ID" width="110" />
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="row.state === 'failed' ? 'danger' : row.state === 'running' ? 'primary' : 'info'" size="small">
                {{ row.state }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="attempt" label="尝试" width="90" align="right" />
          <el-table-column prop="error" label="错误" min-width="260" show-overflow-tooltip />
        </el-table>
      </div>
    </div>

    <div class="oj-card">
      <div class="oj-card-body">
        <p class="muted" style="margin: 0">
          判题任务由 SQLite 队列持久化，执行引擎仅同步执行；服务重启后未完成任务自动恢复。失败任务保留在此，可重新入队。
        </p>
      </div>
    </div>
  </div>
</template>
