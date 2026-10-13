<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, goBack, navigate, refresh, type PublicProblem, type Submission } from '../store'
import StatusTag from '../components/StatusTag.vue'
import LanguageTag from '../components/LanguageTag.vue'

const sub = computed(() => store.selectedSubmission)
const passed = computed(() => (sub.value?.judge_result_json?.cases ?? []).filter((c) => c.verdict === 'AC').length)
const total = computed(() => (sub.value?.judge_result_json?.cases ?? []).length)
const hasCompileLog = computed(() => Boolean(sub.value?.judge_result_json?.compile_log))

function verdictClass(verdict: string) {
  return verdict === 'AC' ? 'oj-log-ok' : verdict === 'WA' ? 'oj-log-bad' : verdict === 'CE' ? 'oj-log-bad' : ''
}

async function openProblem() {
  const id = sub.value?.problem_id
  if (!id) return
  try {
    store.selectedProblem = await api<PublicProblem>(`/problems/${id}`)
    store.editTargetID = id
    navigate('problem-view')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}

let timer: ReturnType<typeof setInterval> | undefined
async function poll() {
  const current = store.selectedSubmission
  if (!current || current.status !== 'waiting') return
  try {
    store.selectedSubmission = await api<Submission>(`/submissions/${current.id}`)
    if (store.selectedSubmission.status !== 'waiting') {
      await refresh()
    }
  } catch {
    /* keep polling */
  }
}
onMounted(() => {
  poll()
  timer = setInterval(poll, 2000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div v-if="sub">
    <div class="oj-toolbar">
      <h2>提交 #{{ sub.id }}</h2>
      <el-tag v-if="sub.status === 'waiting'" type="warning" size="small" effect="light" round>
        判题中，结果自动刷新…
      </el-tag>
      <el-button text type="primary" @click="goBack('submissions')">返回</el-button>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-body">
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="题目">
            <el-link type="primary" @click="openProblem">{{ sub.problem_title }}</el-link>
          </el-descriptions-item>
          <el-descriptions-item label="提交者">{{ sub.submitter }}</el-descriptions-item>
          <el-descriptions-item label="语言"><LanguageTag :language="sub.language" /></el-descriptions-item>
          <el-descriptions-item label="状态"><StatusTag :status="sub.status" /></el-descriptions-item>
          <el-descriptions-item label="分数">
            <span class="mono">{{ sub.score }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="通过点">
            <span v-if="total > 0" class="mono">{{ passed }} / {{ total }}</span>
            <span v-else class="muted">—</span>
          </el-descriptions-item>
          <el-descriptions-item label="提交时间">{{ sub.submitted_at }}</el-descriptions-item>
          <el-descriptions-item label="判题时间">{{ sub.judged_at || '—' }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </div>

    <div v-if="hasCompileLog" class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-header"><h3>编译信息</h3></div>
      <div class="oj-card-body"><pre class="oj-pre oj-log-bad">{{ sub.judge_result_json?.compile_log }}</pre></div>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-header"><h3>逐测试点</h3></div>
      <div class="oj-table-wrap">
        <el-table :data="sub.judge_result_json?.cases ?? []" stripe size="small" border>
          <el-table-column prop="id" label="点" width="80" />
          <el-table-column label="结果" width="110">
            <template #default="{ row }"><StatusTag :status="row.verdict" /></template>
          </el-table-column>
          <el-table-column label="CPU 时间" width="110">
            <template #default="{ row }"><span class="mono">{{ row.time_ms }} ms</span></template>
          </el-table-column>
          <el-table-column label="内存" width="130">
            <template #default="{ row }">
              <span class="mono">{{ (row.memory / 1024).toFixed(0) }} KB</span>
            </template>
          </el-table-column>
          <el-table-column label="信息" min-width="240">
            <template #default="{ row }">
              <span v-if="row.error" :class="verdictClass(row.verdict)">{{ row.error }}</span>
              <span v-else-if="row.stderr" class="muted small">{{ row.stderr }}</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <div class="oj-card">
      <div class="oj-card-header"><h3>源码</h3></div>
      <div class="oj-card-body"><pre class="oj-pre oj-source">{{ sub.source }}</pre></div>
    </div>
  </div>
</template>
