<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, navigate, type Submission } from '../store'
import StatusTag from '../components/StatusTag.vue'
import LanguageTag from '../components/LanguageTag.vue'

type SubmissionPage = {
  items: Submission[]
  total: number
  waiting: number
  page: number
  page_size: number
}

const statusFilter = ref('all')
const languageFilter = ref('all')
const problemFilter = ref<number>(0)
const keyword = ref('')
const loading = ref(false)
const refreshing = ref(false)

const items = ref<Submission[]>([])
const total = ref(0)
const waiting = ref(0)
const page = ref(1)
const pageSize = ref(20)

let debounce: ReturnType<typeof setTimeout> | undefined

async function load(showError = false) {
  refreshing.value = true
  try {
    const params = new URLSearchParams()
    params.set('page', String(page.value))
    params.set('page_size', String(pageSize.value))
    if (statusFilter.value !== 'all') params.set('status', statusFilter.value)
    if (languageFilter.value !== 'all') params.set('language', languageFilter.value)
    if (problemFilter.value) params.set('problem_id', String(problemFilter.value))
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    const data = await api<SubmissionPage>(`/submissions?${params.toString()}`)
    items.value = data.items || []
    total.value = data.total || 0
    waiting.value = data.waiting || 0
  } catch (e) {
    if (showError) ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    refreshing.value = false
  }
}

function resetAndLoad() {
  page.value = 1
  void load(true)
}

watch([statusFilter, languageFilter, problemFilter], resetAndLoad)
watch(keyword, () => {
  if (debounce) clearTimeout(debounce)
  debounce = setTimeout(resetAndLoad, 350)
})

function onPageChange(next: number) {
  page.value = next
  void load()
}
function onSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  void load()
}

let timer: ReturnType<typeof setInterval> | undefined
function poll() {
  if (waiting.value > 0) void load()
}
onMounted(() => {
  void load()
  timer = setInterval(poll, 3000)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  if (debounce) clearTimeout(debounce)
})

async function openDetail(id: number) {
  loading.value = true
  try {
    store.selectedSubmission = await api<Submission>(`/submissions/${id}`)
    navigate('submission-detail')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>
          提交记录 <span class="oj-count">{{ total }}</span>
        </h2>
        <p>查看自测与比赛提交的评测结果</p>
      </div>
      <el-tag v-if="waiting > 0" type="warning" size="small" effect="light" round>
        {{ waiting }} 条等待判题中…
      </el-tag>
      <el-button :loading="refreshing" @click="load(true)">
        <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-filters">
        <el-select v-model="statusFilter" aria-label="筛选状态">
          <el-option label="全部状态" value="all" />
          <el-option label="AC" value="AC" />
          <el-option label="WA" value="WA" />
          <el-option label="CE" value="CE" />
          <el-option label="等待评测" value="waiting" />
        </el-select>
        <el-select v-model="languageFilter" aria-label="筛选语言">
          <el-option label="全部语言" value="all" />
          <el-option label="C++" value="cpp" />
          <el-option label="Java" value="java" />
          <el-option label="Python 3" value="python3" />
        </el-select>
        <el-select v-model="problemFilter" class="wide" aria-label="筛选题目" filterable>
          <el-option label="全部题目" :value="0" />
          <el-option
            v-for="p in store.problems"
            :key="p.id"
            :label="`#${p.id} ${p.title}`"
            :value="p.id"
          />
        </el-select>
        <el-input
          v-model="keyword"
          placeholder="搜索题目 / 提交者"
          aria-label="搜索提交"
          clearable
          class="search-input"
        >
          <template #prefix
            ><el-icon><Search /></el-icon
          ></template>
        </el-input>
      </div>

      <div class="oj-table-wrap">
        <el-table :data="items" v-loading="loading || refreshing" empty-text="没有匹配的提交记录">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="submitter" label="提交者" min-width="130" show-overflow-tooltip />
          <el-table-column
            prop="problem_title"
            label="题目"
            min-width="200"
            show-overflow-tooltip
          />
          <el-table-column label="语言" width="110">
            <template #default="{ row }"><LanguageTag :language="row.language" /></template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }"><StatusTag :status="row.status" /></template>
          </el-table-column>
          <el-table-column prop="score" label="分数" width="100" align="right">
            <template #default="{ row }"
              ><span class="mono">{{ row.score }}</span></template
            >
          </el-table-column>
          <el-table-column prop="submitted_at" label="提交时间" width="170" />
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openDetail(row.id)"
                >详情</el-button
              >
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="oj-pagination">
        <el-pagination
          background
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
          :current-page="page"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          @current-change="onPageChange"
          @size-change="onSizeChange"
        />
      </div>
    </div>
  </div>
</template>
