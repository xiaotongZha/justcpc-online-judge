<script setup lang="ts">
import { computed, toRef, watch } from 'vue'
import { store, navigate } from '../store'
import { useListPagination } from '../listPagination'

const keyword = toRef(store.problemList, 'keyword')
watch(keyword, () => {
  store.problemList.page = 1
})

const filtered = computed(() => {
  const key = keyword.value.trim().toLowerCase()
  return store.problems.filter(
    (p) => !key || p.title.toLowerCase().includes(key) || String(p.id).includes(key),
  )
})

const pageRows = useListPagination(filtered, store.problemList)

function openProblem(id: number) {
  navigate('problem-view', id)
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
        <el-table
          :data="pageRows"
          class="oj-compact-table"
          v-loading="store.loading"
          :empty-text="
            store.problems.length === 0
              ? '题目库还没有题目，点击右上角“新建题目”开始创建'
              : '没有匹配的题目'
          "
        >
          <el-table-column label="编号" width="80">
            <template #default="{ row }"
              ><span class="oj-row-id">#{{ row.id }}</span></template
            >
          </el-table-column>
          <el-table-column label="题目" min-width="280">
            <template #default="{ row }"
              ><button
                class="oj-title-link oj-list-title"
                :title="row.title"
                @click="openProblem(row.id)"
              >
                {{ row.title }}
              </button></template
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
      <div class="oj-pagination oj-list-pagination">
        <el-pagination
          v-model:current-page="store.problemList.page"
          v-model:page-size="store.problemList.pageSize"
          :total="filtered.length"
          :page-sizes="[10, 20, 50, 100]"
          :pager-count="5"
          background
          layout="total, sizes, prev, pager, next"
          aria-label="题目库分页"
        />
      </div>
    </div>
  </div>
</template>
