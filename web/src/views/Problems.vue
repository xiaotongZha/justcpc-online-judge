<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, navigate, type PublicProblem } from '../store'

const keyword = ref('')

const filtered = computed(() => {
  const key = keyword.value.trim().toLowerCase()
  return store.problems.filter((p) => !key || p.title.toLowerCase().includes(key))
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
      <h2>题目</h2>
      <el-button type="primary" @click="newProblem">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建题目
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-filters">
        <el-input v-model="keyword" placeholder="搜索题目标题" clearable style="width: 220px">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>

      <div class="oj-table-wrap">
        <el-table :data="filtered" stripe size="small" v-loading="store.loading">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="title" label="标题" min-width="280" show-overflow-tooltip />
          <el-table-column prop="score" label="分数" width="100" align="right" />
          <el-table-column label="测试数据" width="120">
            <template #default="{ row }">
              <el-tag :type="row.accepting_submissions ? 'success' : 'info'" size="small" effect="light">
                {{ row.accepting_submissions ? '有数据' : '无数据' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90" align="center">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openProblem(row.id)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>
