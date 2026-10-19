<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { downloadTestdataFile } from '../testdataDownload'
import { store, goBack, navigate, refresh, type Submission } from '../store'
import StatusTag from '../components/StatusTag.vue'
import LanguageTag from '../components/LanguageTag.vue'

const sub = computed(() => store.selectedSubmission)
const passed = computed(
  () => (sub.value?.judge_result_json?.cases ?? []).filter((c) => c.verdict === 'AC').length,
)
const total = computed(() => (sub.value?.judge_result_json?.cases ?? []).length)
const hasCompileLog = computed(() => Boolean(sub.value?.judge_result_json?.compile_log))

function downloadCase(caseID: string, kind: 'input' | 'output') {
  if (!sub.value?.can_download_testdata || !store.account) return
  if (!downloadTestdataFile(store.account.id, sub.value.problem_id, caseID, kind)) {
    ElMessage.warning('请勿触发重复下载，请五秒后再试')
  }
}

async function openProblem() {
  const id = sub.value?.problem_id
  if (!id) return
  navigate('problem-view', id)
}

let timer: ReturnType<typeof setInterval> | undefined
async function poll() {
  const current = store.selectedSubmission
  if (!current || current.status !== 'waiting') return
  try {
    const item = await api<Submission>(`/submissions/${current.id}`)
    if (store.tab !== 'submission-detail' || store.routeID !== current.id) return
    store.selectedSubmission = item
    if (item.status !== 'waiting') {
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
      <div class="oj-heading-copy">
        <h2>提交 #{{ sub.id }}</h2>
        <p>评测结果与逐测试点记录</p>
      </div>
      <el-tag v-if="sub.status === 'waiting'" type="warning" size="small">结果自动刷新</el-tag>
      <el-button @click="goBack('submissions')"
        ><el-icon><ArrowLeft /></el-icon>返回</el-button
      >
    </div>
    <div class="oj-card">
      <div class="oj-card-body">
        <div class="oj-result-summary">
          <StatusTag :status="sub.status" />
          <div class="oj-result-score">
            {{ sub.status === 'waiting' ? '—' : sub.score }}<span>分</span>
          </div>
          <span v-if="sub.status !== 'waiting' && total > 0" class="oj-result-passed"
            >通过 <strong class="mono">{{ passed }} / {{ total }}</strong> 个测试点</span
          >
          <span v-else-if="sub.status === 'waiting'" class="oj-result-passed">等待评测完成</span>
        </div>
        <dl class="oj-details-grid">
          <div>
            <dt>题目</dt>
            <dd>
              <el-link type="primary" @click="openProblem">{{ sub.problem_title }}</el-link>
            </dd>
          </div>
          <div>
            <dt>提交者</dt>
            <dd>{{ sub.submitter }}</dd>
          </div>
          <div>
            <dt>语言</dt>
            <dd><LanguageTag :language="sub.language" /></dd>
          </div>
          <div>
            <dt>提交时间</dt>
            <dd>{{ sub.submitted_at }}</dd>
          </div>
          <div>
            <dt>判题时间</dt>
            <dd>{{ sub.judged_at || '—' }}</dd>
          </div>
        </dl>
      </div>
    </div>
    <div v-if="hasCompileLog" class="oj-card">
      <div class="oj-card-header">
        <h3>编译信息</h3>
        <el-tag type="danger" size="small">编译诊断</el-tag>
      </div>
      <div class="oj-card-body">
        <pre class="oj-pre oj-log-bad">{{ sub.judge_result_json?.compile_log }}</pre>
      </div>
    </div>
    <div class="oj-card">
      <div class="oj-card-header">
        <h3>逐测试点</h3>
        <span v-if="total" class="small muted">共 {{ total }} 个测试点</span>
      </div>
      <div v-if="!total" class="oj-empty-state">
        {{
          sub.status === 'CE'
            ? '编译失败，未执行测试点'
            : sub.status === 'waiting'
              ? '评测完成后显示测试点结果'
              : '暂无测试点结果'
        }}
      </div>
      <div v-else class="oj-table-wrap">
        <el-table :data="sub.judge_result_json?.cases ?? []">
          <el-table-column prop="id" label="测试点" min-width="100" />
          <el-table-column label="结果" min-width="110"
            ><template #default="{ row }"><StatusTag :status="row.verdict" /></template
          ></el-table-column>
          <el-table-column label="CPU 时间" min-width="130"
            ><template #default="{ row }"
              ><span class="mono">{{ row.time_ms }} ms</span></template
            ></el-table-column
          >
          <el-table-column label="内存" min-width="140"
            ><template #default="{ row }"
              ><span class="mono">{{ (row.memory / 1024).toFixed(0) }} KB</span></template
            ></el-table-column
          >
          <el-table-column v-if="sub.can_download_testdata" label="下载输入" min-width="140">
            <template #default="{ row }">
              <el-button link type="primary" @click="downloadCase(row.id, 'input')">
                <el-icon><Download /></el-icon>下载输入
              </el-button>
            </template>
          </el-table-column>
          <el-table-column v-if="sub.can_download_testdata" label="下载输出" min-width="140">
            <template #default="{ row }">
              <el-button link type="primary" @click="downloadCase(row.id, 'output')">
                <el-icon><Download /></el-icon>下载输出
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
    <div class="oj-card">
      <div class="oj-card-header">
        <h3>提交源码</h3>
        <LanguageTag :language="sub.language" />
      </div>
      <div class="oj-card-body">
        <pre class="oj-pre">{{ sub.source }}</pre>
      </div>
    </div>
  </div>
</template>
