<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, goBack, navigate, refresh } from '../store'
import type { ContestDetail } from '../store'
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
const previewing = ref(false)
const exportingExam = ref(false)
const exportDialog = ref(false)
const exportedFile = ref('')
const importDialog = ref(false)
const importComplete = ref(false)
const importedCount = ref(0)
const evaluationPanel = ref<HTMLElement | null>(null)
const importConfirmed = ref(false)
const importFile = ref<File | null>(null)
type ImportPreview = {
  name: string
  target_name: string
  contest_id: number
  package_contest_id: number
  contestants: number
  submissions: number
  requires_mapping: boolean
  problems: { label: string; title: string; package_problem_id: number; problem_id: number }[]
}
const importPreview = ref<ImportPreview | null>(null)
let importRequest = 0
let timer: number | undefined

const contestId = computed(() => store.selectedContest?.id ?? 0)
const hasData = computed(() => evaluation.value.rows.length > 0)
const hasSubmissions = computed(() =>
  evaluation.value.rows.some((row) => row.cells.some((cell) => cell.has_submission)),
)
const waitingCount = computed(() =>
  evaluation.value.rows.reduce(
    (count, row) =>
      count + row.cells.filter((cell) => cell.has_submission && cell.status === 'waiting').length,
    0,
  ),
)
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
  detail.value = store.selectedContest as ContestDetail
  loadEvaluation()
  timer = window.setInterval(loadEvaluation, 2000)
})
onUnmounted(() => {
  importRequest++
  if (timer) window.clearInterval(timer)
})

// ---------- 单元格选择 ----------
const keyOf = (ri: number, ci: number) =>
  String(evaluation.value.rows[ri]?.cells[ci]?.submission_id ?? 0)

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
  navigate('submission-detail', cell.submission_id)
}

async function enqueueSelected() {
  const currentIDs = new Set(
    evaluation.value.rows.flatMap((row) => row.cells.map((cell) => cell.submission_id)),
  )
  const ids = [...selected.value].map(Number).filter((id) => id > 0 && currentIDs.has(id))
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
  const disposition = response.headers.get('Content-Disposition') || ''
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(disposition)
  const plain = /filename="?([^";]+)"?/i.exec(disposition)
  anchor.download = encoded ? decodeURIComponent(encoded[1]) : plain?.[1] || filename
  anchor.click()
  URL.revokeObjectURL(url)
  return anchor.download
}

async function exportExam() {
  if (!contestId.value || exportingExam.value) return
  exportingExam.value = true
  try {
    exportedFile.value = await downloadPost(
      `/contests/${contestId.value}/export-exam`,
      `exam-package-${contestId.value}.zip`,
    )
    exportDialog.value = true
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    exportingExam.value = false
  }
}

async function onImportFile(file: any) {
  if (!contestId.value || !file?.raw || importing.value) return
  const request = ++importRequest
  previewing.value = true
  importConfirmed.value = false
  importPreview.value = null
  importFile.value = file.raw
  const data = new FormData()
  data.append('file', file.raw)
  try {
    const preview = await api<ImportPreview>(`/contests/${contestId.value}/preview-submissions`, {
      method: 'POST',
      body: data,
    })
    if (request !== importRequest) return
    importPreview.value = preview
    importDialog.value = true
  } catch (error) {
    if (request === importRequest)
      ElMessage.error(error instanceof Error ? error.message : String(error))
  } finally {
    if (request === importRequest) previewing.value = false
  }
}

async function confirmImport() {
  const preview = importPreview.value
  if (!preview || !importFile.value || !importConfirmed.value || importing.value) return
  importing.value = true
  const data = new FormData()
  data.append('file', importFile.value)
  if (preview.requires_mapping)
    data.append(
      'mapping',
      JSON.stringify({
        contest_id: contestId.value,
        problems: Object.fromEntries(preview.problems.map((p) => [p.label, p.problem_id])),
      }),
    )
  try {
    const result = await api<{ contestants: number; submissions: number }>(
      `/contests/${contestId.value}/import-submissions`,
      { method: 'POST', body: data },
    )
    importDialog.value = false
    importFile.value = null
    exitFilter()
    importedCount.value = result.submissions
    await reloadAll()
    importComplete.value = true
    await refresh().catch(() => ElMessage.warning('提交包已导入，列表更新失败，请刷新状态'))
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error))
  } finally {
    importing.value = false
  }
}
async function chooseImportedCode() {
  importComplete.value = false
  enterFilter()
  await nextTick()
  evaluationPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<template>
  <div v-if="detail || store.selectedContest">
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>{{ detail?.name || store.selectedContest?.name }}</h2>
        <p>
          {{ detail?.problem_count ?? 0 }} 道题 · {{ detail?.contestant_count ?? 0 }} 位选手<span
            v-if="detail?.description"
          >
            · {{ detail.description }}</span
          >
        </p>
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

    <section
      v-if="store.account?.role === 'admin'"
      class="contest-handoff"
      aria-label="考试文件交接"
    >
      <div class="handoff-action">
        <span class="handoff-direction">考前 · 交给考场</span>
        <h3>准备考试文件</h3>
        <p>题面和选手名单，一次交给监考人员。</p>
        <el-button
          :type="hasSubmissions ? 'default' : 'primary'"
          :loading="exportingExam"
          @click="exportExam"
          ><el-icon><Download /></el-icon>导出考试包</el-button
        >
      </div>
      <div class="handoff-action">
        <span class="handoff-direction">考后 · 收回代码</span>
        <h3>接收选手提交</h3>
        <p>选择监考端导出的提交包，核对后测评。</p>
        <el-upload
          :disabled="importing || previewing"
          :auto-upload="false"
          :show-file-list="false"
          accept=".zip"
          :on-change="onImportFile"
        >
          <el-button
            :type="hasSubmissions ? 'primary' : 'default'"
            :loading="importing || previewing"
            ><el-icon><Upload /></el-icon>导入提交包</el-button
          >
        </el-upload>
      </div>
    </section>

    <el-dialog
      v-model="exportDialog"
      title="考试包已下载"
      width="min(540px, 95vw)"
      :close-on-click-modal="false"
    >
      <div class="downloaded-exam">
        <el-icon><Document /></el-icon>
        <div>
          <strong>{{ exportedFile }}</strong
          ><span>考试包 · 保留 ZIP 原样，无需解压</span>
        </div>
      </div>
      <div class="export-next">
        <span>下一步 · 交给监考人员</span>
        <h3>在考场电脑导入这份文件</h3>
        <p>把 ZIP 文件复制到考场电脑，打开考试系统首页，点击「导入考试包」。</p>
      </div>
      <template #footer
        ><el-button type="primary" @click="exportDialog = false">知道了</el-button></template
      >
    </el-dialog>

    <el-dialog
      v-model="importDialog"
      title="预览提交包，核对后导入"
      width="min(720px, 95vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!importing"
      :show-close="!importing"
    >
      <template v-if="importPreview">
        <p>文件：{{ importFile?.name }}</p>
        <p>
          包内比赛：{{ importPreview.name || '未提供名称' }} · ID
          {{ importPreview.package_contest_id || '未提供' }}
        </p>
        <p>
          <strong
            >目标比赛：{{ importPreview.target_name }} · #{{ importPreview.contest_id }}</strong
          >
        </p>
        <p>
          {{ importPreview.contestants }} 位选手 · {{ importPreview.problems.length }} 道题 ·
          {{ importPreview.submissions }} 份有效代码
        </p>
        <el-alert
          v-if="importPreview.requires_mapping"
          title="历史包缺少平台 ID，无法自动证明属于本场比赛。请逐项核对下面的题号对应关系，再确认导入。"
          type="warning"
          :closable="false"
          show-icon
        />
        <el-table :data="importPreview.problems" max-height="300">
          <el-table-column prop="label" label="包内题号" width="100" />
          <el-table-column prop="package_problem_id" label="包内题目 ID" width="120" />
          <el-table-column prop="problem_id" label="OJ 题目 ID" width="120" />
          <el-table-column prop="title" label="将导入到的题目" />
        </el-table>
        <el-checkbox
          v-model="importConfirmed"
          :disabled="importing"
          style="margin-top: 16px; white-space: normal; height: auto"
          >我已核对目标比赛、选手人数及全部题号对应关系，确认导入</el-checkbox
        >
        <p class="small muted">导入只收录代码；完成后在成绩表中选择提交并加入测评队列。</p>
      </template>
      <template #footer
        ><el-button :disabled="importing" @click="importDialog = false">取消</el-button
        ><el-button
          type="primary"
          :disabled="!importConfirmed"
          :loading="importing"
          @click="confirmImport"
          >确认导入提交包</el-button
        ></template
      >
    </el-dialog>

    <el-dialog
      v-model="importComplete"
      title="导入完成，尚未开始测评"
      width="min(520px, 95vw)"
      :close-on-click-modal="false"
    >
      <div class="import-complete-copy">
        <span class="import-complete-count">{{ importedCount }} <small>份代码已收录</small></span>
        <h3>下一步：选择代码测评</h3>
        <p>导入不会自动开始测评。请在成绩表中勾选代码，再点击「加入测评队列」。</p>
      </div>
      <template #footer>
        <el-button @click="importComplete = false">稍后测评</el-button>
        <el-button
          type="primary"
          :disabled="!hasSubmissions || evaluation.pending > 0"
          @click="chooseImportedCode"
          >选择代码测评</el-button
        >
      </template>
    </el-dialog>

    <div ref="evaluationPanel" class="oj-card evaluation-panel">
      <div class="oj-card-header eval-header">
        <h3>评测成绩表</h3>
        <a :href="`/api/contests/${contestId}/statement.pdf`" target="_blank" class="result-link"
          >题面 PDF</a
        >
        <el-dropdown v-if="hasSubmissions" trigger="click"
          ><el-button text
            >导出结果<el-icon><ArrowDown /></el-icon></el-button
          ><template #dropdown
            ><el-dropdown-menu
              ><el-dropdown-item
                ><a :href="`/api/contests/${contestId}/scoreboard.csv`" class="link-plain"
                  >成绩单 CSV</a
                ></el-dropdown-item
              ><el-dropdown-item
                ><a :href="`/api/contests/${contestId}/result-package`" class="link-plain"
                  >完整结果包</a
                ></el-dropdown-item
              ></el-dropdown-menu
            ></template
          ></el-dropdown
        >
        <span v-if="evaluation.pending > 0" class="pending-badge">
          <el-icon class="is-loading"><Loading /></el-icon>正在评测 {{ evaluation.pending }} 条…
        </span>
        <span class="spacer" />
        <template v-if="!filterMode">
          <el-button
            type="primary"
            :disabled="evaluation.pending > 0 || !hasSubmissions"
            @click="enterFilter"
            >选择代码测评</el-button
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

      <div v-if="filterMode" class="evaluation-next" role="status">
        <strong>选择要测评的代码</strong>
        <span>勾选单元格、整行或整列，也可全选；确认后点击「加入测评队列」。</span>
      </div>
      <div
        v-else-if="evaluation.pending === 0 && waitingCount > 0"
        class="evaluation-next"
        role="status"
      >
        <strong>{{ waitingCount }} 份代码尚未开始测评</strong>
        <span>点击「选择代码测评」，确认需要测评的提交后再加入队列。</span>
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
          <el-empty description="尚未导入提交包，请先在考场 Exam 导出提交包" :image-size="80" />
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
.evaluation-panel {
  scroll-margin-top: 24px;
}
.evaluation-next {
  display: flex;
  gap: 12px 20px;
  flex-wrap: wrap;
  padding: 14px 24px;
  border-bottom: 1px solid var(--oj-border);
  background: #f8f9fd;
  font-size: 13px;
}
.evaluation-next strong {
  color: var(--oj-text-primary);
}
.evaluation-next span {
  color: var(--oj-text-secondary);
}
.import-complete-copy {
  padding: 8px 4px;
}
.import-complete-count {
  display: block;
  font-size: 32px;
  color: var(--el-color-primary);
  font-weight: 650;
  margin-bottom: 20px;
}
.import-complete-count small {
  font-size: 14px;
  color: var(--oj-text-secondary);
  font-weight: 400;
}
.import-complete-copy h3 {
  font-size: 20px;
  margin: 8px 0;
}
.import-complete-copy p {
  color: var(--oj-text-secondary);
  line-height: 1.8;
}

.contest-handoff {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
  margin-bottom: 22px;
}
.handoff-action {
  background: white;
  border: 1px solid var(--oj-border);
  border-radius: 12px;
  padding: 24px;
}
.handoff-direction {
  font-size: 12px;
  color: var(--oj-text-secondary);
}
.handoff-action h3 {
  margin: 8px 0;
  font-size: 20px;
}
.handoff-action p {
  color: var(--oj-text-secondary);
  font-size: 13px;
  margin: 0 0 20px;
}
.handoff-action .el-icon {
  margin-right: 6px;
}
.downloaded-exam {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--oj-border);
  border-radius: 10px;
  background: #fafbfe;
}
.downloaded-exam > .el-icon {
  width: 42px;
  height: 48px;
  font-size: 26px;
  color: var(--el-color-primary);
  background: #eef1ff;
  border-radius: 8px;
  flex: none;
}
.downloaded-exam > div {
  min-width: 0;
}
.downloaded-exam strong {
  overflow-wrap: anywhere;
}
.downloaded-exam span {
  display: block;
  color: var(--oj-text-secondary);
  font-size: 12px;
  margin-top: 6px;
}
.export-next {
  margin-top: 24px;
}
.export-next > span {
  color: var(--oj-text-secondary);
  font-size: 12px;
}
.export-next h3 {
  font-size: 20px;
  margin: 8px 0;
}
.export-next p {
  color: var(--oj-text-secondary);
}
.result-link {
  font-size: 12px;
  margin-left: 8px;
}
@media (max-width: 800px) {
  .contest-handoff {
    grid-template-columns: 1fr;
  }
}

.contest-heading-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.link-plain {
  color: inherit;
  text-decoration: none;
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
  .eval-table .col-name {
    position: static;
  }
  .eval-scroll {
    max-height: 65vh;
  }
}
</style>
