<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, navigate, type PublicProblem } from '../store'

const keyword = ref('')

const filtered = computed(() => {
  const key = keyword.value.trim().toLowerCase()
  return store.problems.filter(
    (p) => !key || p.title.toLowerCase().includes(key) || String(p.id).includes(key),
  )
})

async function openProblem(id: number) {
  try {
    store.loading = true
    store.selectedProblem = await api<PublicProblem>(`/problems/${id}`)
    store.editTargetID = id
    navigate('problem-view')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    store.loading = false
  }
}

function newProblem() {
  store.editTargetID = 0
  navigate('problem-edit')
}
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>
          题目库 <span class="oj-count">{{ store.problems.length }}</span>
        </h2>
        <p>管理题面、资源限制与测试数据</p>
      </div>
      <el-button type="primary" @click="newProblem">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建题目
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-filters">
        <span class="filter-label">全部题目</span>
        <el-input
          v-model="keyword"
          placeholder="搜索题目名称或 ID"
          aria-label="搜索题目"
          clearable
          class="search-input"
        >
          <template #prefix
            ><el-icon><Search /></el-icon
          ></template>
        </el-input>
      </div>

      <div class="oj-table-wrap">
        <el-table :data="filtered" v-loading="store.loading" empty-text="没有匹配的题目">
          <el-table-column label="题目" min-width="280">
            <template #default="{ row }"
              ><button class="oj-title-link" @click="openProblem(row.id)">{{ row.title }}</button
              ><span class="oj-table-id">#{{ row.id }}</span></template
            >
          </el-table-column>
          <el-table-column prop="score" label="分值" width="120" align="right" />
          <el-table-column label="测试数据" width="170">
            <template #default="{ row }">
              <span
                class="oj-data-state"
                :class="row.accepting_submissions ? 'ready' : 'incomplete'"
                >{{ row.accepting_submissions ? '已就绪' : '待补充' }}</span
              >
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" align="center" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openProblem(row.id)"
                >详情</el-button
              >
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div class="oj-card-footer">共 {{ filtered.length }} 道题目</div>
    </div>
  </div>
</template>
