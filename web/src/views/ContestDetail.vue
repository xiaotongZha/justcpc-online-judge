<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, goBack, navigate, refresh } from '../store'
import type { ContestDetail, Submission } from '../store'
import { deletingContestID, removeContest } from '../contestActions'

type EvalProblem = { problem_id: number; label: string; title: string; score: string }
type EvalCell = { has_submission: boolean; submission_id?: number; status?: string; score?: string }
type EvalRow = {
  rank: number
  has_score: boolean
  student_id: string
  name: string
  cells: EvalCell[]
  total: string
}
type Evaluation = { contest_id: number; problems: EvalProblem[]; rows: EvalRow[]; pending: number }

const detail = ref<ContestDetail | null>(null)
const evaluation = ref<Evaluation>({ contest_id: 0, problems: [], rows: [], pending: 0 })
const filterMode = ref(false)
const selected = ref<Set<string>>(new Set())
const enqueuing = ref(false)
const importing = ref(false)
let timer: number | undefined

const contestId = computed(() => store.selectedContest?.id ?? 0)
const hasData = computed(() => evaluation.value.rows.length > 0)
const selectedCount = computed(() => selected.value.size)

async function loadDetail() {
  if (!contestId.value) return
  try {
    detail.value = await api<ContestDetail>(`/contests/${contestId.value}`)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}

async function loadEvaluation() {
  if (!contestId.value) return
  try {
    evaluation.value = await api<Evaluation>(`/contests/${contestId.value}/evaluation`)
  } catch {
    /* 评测期间忽略瞬时错误，下一轮再刷新 */
  }
}

async function reloadAll() {
  await Promise.all([loadDetail(), loadEvaluation()])
}

onMounted(() => {
  reloadAll()
  timer = window.setInterval(loadEvaluation, 2000)
})
onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

// ---------- 单元格选择 ----------
const keyOf = (ri: number, ci: number) => `${ri}:${ci}`

function setSelected(keys: string[], on: boolean) {
  const next = new Set(selected.value)
  for (const key of keys) {
    if (on) next.add(key)
    else next.delete(key)
  }
  selected.value = next
}
function rowKeys(ri: number): string[] {
  const keys: string[] = []
  evaluation.value.rows[ri]?.cells.forEach((cell, ci) => {
    if (cell.has_submission) keys.push(keyOf(ri, ci))
  })
  return keys
}
function colKeys(ci: number): string[] {
  const keys: string[] = []
  evaluation.value.rows.forEach((row, ri) => {
    if (row.cells[ci]?.has_submission) keys.push(keyOf(ri, ci))
  })
  return keys
}
function allKeys(): string[] {
  const keys: string[] = []
  evaluation.value.rows.forEach((row, ri) =>
    row.cells.forEach((cell, ci) => {
      if (cell.has_submission) keys.push(keyOf(ri, ci))
    }),
  )
  return keys
}
const isSelected = (ri: number, ci: number) => selected.value.has(keyOf(ri, ci))
const rowSelected = (ri: number) => {
  const keys = rowKeys(ri)
  return keys.length > 0 && keys.every((key) => selected.value.has(key))
}
const colSelected = (ci: number) => {
  const keys = colKeys(ci)
  return keys.length > 0 && keys.every((key) => selected.value.has(key))
}
const allSelected = computed(() => {
  const keys = allKeys()
  return keys.length > 0 && keys.every((key) => selected.value.has(key))
})

function toggleCell(ri: number, ci: number) {
  if (!evaluation.value.rows[ri]?.cells[ci]?.has_submission) return
  setSelected([keyOf(ri, ci)], !isSelected(ri, ci))
}
function toggleRow(ri: number) {
  setSelected(rowKeys(ri), !rowSelected(ri))
}
function toggleColumn(ci: number) {
  setSelected(colKeys(ci), !colSelected(ci))
}
function selectAll() {
  setSelected(allKeys(), !allSelected.value)
}

function enterFilter() {
  if (evaluation.value.pending > 0) return
  selected.value = new Set()
  filterMode.value = true
}
function exitFilter() {
  filterMode.value = false
  selected.value = new Set()
}

// 单元格颜色按得分：满分绿、部分分橙、0 分红、未判中性色。
function cellTone(cell: EvalCell, ci: number) {
  if (!cell.has_submission) return ''
  if (cell.status === 'waiting') return 'tone-waiting'
  const max = Number(evaluation.value.problems[ci]?.score ?? 0)
  const got = Number(cell.score ?? 0)
  if (max > 0 && got >= max) return 'tone-full'
  if (got > 0) return 'tone-partial'
  return 'tone-zero'
}

// 点击单元格跳转到该提交的测评记录；筛选模式下禁用，避免与勾选冲突。
async function openSubmission(cell: EvalCell) {
  if (filterMode.value || !cell?.submission_id) return
  try {
    store.selectedSubmission = await api<Submission>(`/submissions/${cell.submission_id}`)
    navigate('submission-detail')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}

async function enqueueSelected() {
  const ids: number[] = []
  for (const key of selected.value) {
    const [ri, ci] = key.split(':').map(Number)
    const cell = evaluation.value.rows[ri]?.cells[ci]
    if (cell?.submission_id) ids.push(cell.submission_id)
  }
  if (!ids.length) return
  enqueuing.value = true
  try {
    const result = await api<{ enqueued: number }>(`/contests/${contestId.value}/enqueue`, {
      method: 'POST',
      body: JSON.stringify({ submission_ids: ids }),
    })
    ElMessage.success(`已加入 ${result.enqueued} 条提交到评测队列`)
    exitFilter()
    await loadEvaluation()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    enqueuing.value = false
  }
}

async function downloadPost(path: string, filename: string) {
  const response = await fetch('/api' + path, { method: 'POST', credentials: 'same-origin' })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || `下载失败：${response.status}`)
  }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

async function exportExam() {
  if (!contestId.value) return
  try {
    await downloadPost(
      `/contests/${contestId.value}/export-exam`,
      `exam-package-${contestId.value}.zip`,
    )
    ElMessage.success('考试包已导出')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}

async function onImportFile(file: any) {
  if (!contestId.value || !file?.raw) return
  importing.value = true
  const data = new FormData()
  data.append('file', file.raw)
  try {
    const result = await api<{ contestants: number; submissions: number }>(
      `/contests/${contestId.value}/import-submissions`,
      {
        method: 'POST',
        body: data,
      },
    )
    ElMessage.success(`已导入：新增考生 ${result.contestants}，提交 ${result.submissions}`)
    await reloadAll()
    await refresh()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div v-if="detail || store.selectedContest">
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>{{ detail?.name || store.selectedContest?.name }}</h2>
        <p>{{ detail?.description || '考试数据管理与批量评测' }}</p>
      </div>
      <div class="contest-heading-actions">
        <el-button text type="primary" @click="goBack('contests')">返回</el-button>
        <el-button
          v-if="store.account?.role === 'admin' && store.selectedContest"
          text
          type="danger"
          :loading="deletingContestID === contestId"
          :disabled="
            importing ||
            enqueuing ||
            (deletingContestID !== null && deletingContestID !== contestId)
          "
          @click="removeContest(detail || store.selectedContest)"
          >删除比赛</el-button
        >
      </div>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-body">
        <div class="action-bar">
          <div class="action-group">
            <span class="group-label">考试数据</span>
            <el-button @click="exportExam">导出考试包</el-button>
            <el-upload
              :auto-upload="false"
              :show-file-list="false"
              accept=".zip"
              :on-change="onImportFile"
            >
              <el-button type="primary" :loading="importing">导入考试数据包</el-button>
            </el-upload>
          </div>
          <div class="action-sep" />
          <div class="action-group">
            <span class="group-label">结果输出</span>
            <el-button :disabled="!hasData">
              <a :href="`/api/contests/${contestId}/scoreboard.csv`" class="link-plain"
                >导出成绩单 (CSV)</a
              >
            </el-button>
            <el-button :disabled="!hasData">
              <a :href="`/api/contests/${contestId}/result-package`" class="link-plain"
                >导出结果包</a
              >
            </el-button>
            <a
              :href="`/api/contests/${contestId}/statement.pdf`"
              target="_blank"
              class="el-link el-link--primary"
              >查看题面 PDF</a
            >
          </div>
        </div>
        <div class="meta-row">
          <el-tag size="small" type="info"
            >题目 {{ detail?.problem_count ?? evaluation.problems.length }}</el-tag
          >
          <el-tag size="small" type="info"
            >考生 {{ detail?.contestant_count ?? evaluation.rows.length }}</el-tag
          >
        </div>
      </div>
    </div>

    <div class="oj-card">
      <div class="oj-card-header eval-header">
        <h3>评测成绩表</h3>
        <span v-if="evaluation.pending > 0" class="pending-badge">
          <el-icon class="is-loading"><Loading /></el-icon>正在评测 {{ evaluation.pending }} 条…
        </span>
        <span class="spacer" />
        <template v-if="!filterMode">
          <el-button
            type="primary"
            :disabled="evaluation.pending > 0 || !hasData"
            @click="enterFilter"
            >筛选测评</el-button
          >
        </template>
        <template v-else>
          <el-checkbox :model-value="allSelected" @change="selectAll">全选</el-checkbox>
          <el-button @click="exitFilter">取消</el-button>
          <el-button
            type="primary"
            :disabled="selectedCount === 0"
            :loading="enqueuing"
            @click="enqueueSelected"
          >
            加入测评队列（{{ selectedCount }}）
          </el-button>
        </template>
      </div>

      <div class="eval-scroll" tabindex="0" aria-label="评测成绩表，可横向滚动">
        <table
          v-if="hasData"
          class="eval-table"
          :class="{ filtering: filterMode }"
          :style="{ minWidth: `${428 + evaluation.problems.length * 96}px` }"
        >
          <thead>
            <tr>
              <th class="col-rank">排名</th>
              <th class="col-id">学号</th>
              <th class="col-name">姓名</th>
              <th v-for="(p, ci) in evaluation.problems" :key="p.problem_id" class="col-prob">
                <div class="th-inner">
                  <el-checkbox
                    v-if="filterMode"
                    :model-value="colSelected(ci)"
                    @change="toggleColumn(ci)"
                  />
                  <el-tooltip :content="p.title"
                    ><span class="prob-label">{{ p.label }}</span></el-tooltip
                  >
                  <span class="prob-score">{{ p.score }}分</span>
                </div>
              </th>
              <th class="col-total">总分</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, ri) in evaluation.rows" :key="row.student_id">
              <td class="col-rank">{{ row.has_score ? row.rank : '-' }}</td>
              <td class="col-id">
                <div class="th-inner">
                  <el-checkbox
                    v-if="filterMode"
                    :model-value="rowSelected(ri)"
                    @change="toggleRow(ri)"
                  />
                  <span class="mono">{{ row.student_id }}</span>
                </div>
              </td>
              <td class="col-name">{{ row.name }}</td>
              <td
                v-for="(cell, ci) in row.cells"
                :key="ci"
                class="col-prob cell"
                :class="[
                  cellTone(cell, ci),
                  { 'cell-clickable': !filterMode && cell.has_submission },
                ]"
                @click="openSubmission(cell)"
              >
                <span v-if="cell.has_submission && cell.status === 'waiting'" class="cell-waiting"
                  >待测评</span
                >
                <span v-else-if="cell.has_submission" class="cell-score">{{ cell.score }}</span>
                <span v-else class="cell-empty" aria-label="未提交">—</span>
                <el-checkbox
                  v-if="filterMode && cell.has_submission"
                  class="cell-check"
                  :model-value="isSelected(ri, ci)"
                  @change="toggleCell(ri, ci)"
                />
              </td>
              <td class="col-total">{{ row.total }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty-hint">
          <el-empty description="尚未导入考试数据" :image-size="80" />
        </div>
      </div>
      <div v-if="hasData" class="score-legend">
        <span><i class="legend-full" />满分</span><span><i class="legend-partial" />部分得分</span
        ><span><i class="legend-zero" />零分</span><span><i class="legend-waiting" />待评测</span
        ><span class="muted">— 未提交</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.contest-heading-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.action-bar {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
}
.action-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.action-group .el-button + .el-button,
.action-group .el-upload {
  margin-left: 0;
}
.group-label {
  font-size: 12.5px;
  color: var(--oj-text-secondary);
  margin-right: 2px;
}
.action-sep {
  width: 1px;
  align-self: stretch;
  background: #e4e7ed;
}
.link-plain {
  color: inherit;
  text-decoration: none;
}
.meta-row {
  display: flex;
  gap: 10px;
  margin-top: 14px;
}
.eval-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.eval-header h3 {
  margin: 0;
}
.eval-header .spacer {
  flex: 1;
}
.pending-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #1c6fe8;
}
.eval-scroll {
  overflow: auto;
  max-height: 68vh;
  border-radius: 0 0 8px 8px;
}
.eval-table {
  width: 100%;
  table-layout: fixed;
  border-spacing: 0;
}
.eval-table th,
.eval-table td {
  border-right: 1px solid var(--oj-border);
  border-bottom: 1px solid var(--oj-border);
  padding: 10px 8px;
  font-size: 13px;
  text-align: center;
  height: 46px;
}
.eval-table th {
  position: sticky;
  top: 0;
  z-index: 3;
  background: #f6f8fa;
  color: #48525c;
  font-weight: 600;
  white-space: nowrap;
}
.eval-table .col-id,
.eval-table .col-name {
  text-align: left;
  position: sticky;
  z-index: 2;
  background: #fff;
}
.eval-table th.col-id,
.eval-table th.col-name,
.eval-table th.col-total {
  z-index: 4;
  background: #f6f8fa;
}
.col-rank {
  width: 60px;
}
.col-id {
  width: 150px;
  left: 0;
}
.col-name {
  width: 128px;
  left: 150px;
}
.col-total {
  width: 90px;
  font-weight: 600;
  position: sticky;
  right: 0;
  z-index: 2;
  background: #f8fafc;
  box-shadow: -1px 0 0 var(--oj-border);
}
.col-prob {
  width: 96px;
}
.th-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}
.col-id .th-inner {
  justify-content: flex-start;
}
.prob-label {
  font-weight: 600;
}
.prob-score {
  color: var(--oj-text-secondary);
  font-size: 12px;
}
.cell {
  position: relative;
}
.cell-waiting {
  color: #c77700;
  font-weight: 600;
}
.cell-score {
  font-family: ui-monospace, Consolas, monospace;
}
.tone-full {
  background: #eefaf2;
}
.tone-partial {
  background: #fdf6e9;
}
.tone-zero {
  background: #fdeeee;
}
.tone-waiting {
  background: #fffdf5;
}
.cell-clickable {
  cursor: pointer;
}
.cell-clickable:hover {
  background: #eaf1fd;
  box-shadow: inset 0 0 0 1px #bcd6f7;
}
.cell-check {
  position: absolute;
  top: 5px;
  left: 6px;
  height: auto;
  margin: 0;
}
.empty-hint {
  padding: 24px;
}
.cell-empty {
  color: #a0a9b7;
  font-size: 12px;
}
.filtering .cell {
  padding-top: 18px;
  height: 54px;
}
.score-legend {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 20px;
  padding: 14px 22px;
  font-size: 12px;
  color: var(--oj-text-secondary);
}
.score-legend > span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.score-legend i {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}
.legend-full {
  background: #bee2cd;
}
.legend-partial {
  background: #efd7a4;
}
.legend-zero {
  background: #edc1c6;
}
.legend-waiting {
  background: #f5eac0;
}
@media (max-width: 760px) {
  .action-sep {
    display: none;
  }
  .eval-table .col-name {
    position: static;
  }
  .eval-scroll {
    max-height: 65vh;
  }
}
</style>
