<script setup lang="ts">
import { computed } from 'vue'
import { store, navigate } from '../store'
import { useListPagination } from '../listPagination'
import type { Contest } from '../store'

const pageRows = useListPagination(
  computed(() => store.contests),
  store.contestList,
)

function openContest(item: Contest) {
  store.selectedContest = item
  navigate('contest-detail')
}
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>
          比赛管理 <span class="oj-count">{{ store.contests.length }}</span>
        </h2>
        <p>组卷、管理考试数据与批量评测</p>
      </div>
      <el-button type="primary" @click="navigate('contest-create')">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建比赛
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-table-wrap">
        <el-table
          class="oj-compact-table"
          :data="pageRows"
          v-loading="store.loading"
          empty-text="尚未创建比赛"
        >
          <el-table-column label="编号" width="80">
            <template #default="{ row }"
              ><span class="oj-row-id">#{{ row.id }}</span></template
            >
          </el-table-column>
          <el-table-column label="比赛" min-width="260">
            <template #default="{ row }"
              ><button
                class="oj-title-link oj-list-title"
                :title="row.name"
                @click="openContest(row)"
              >
                {{ row.name }}
              </button></template
            >
          </el-table-column>
          <el-table-column label="题数" width="80" align="right">
            <template #default="{ row }">{{ row.problem_count }}</template>
          </el-table-column>
          <el-table-column label="考生" width="80" align="right">
            <template #default="{ row }">{{ row.contestant_count }}</template>
          </el-table-column>
          <el-table-column label="提交" width="80" align="right">
            <template #default="{ row }">{{ row.submission_count }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="170" />
          <el-table-column label="操作" width="90" align="center" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openContest(row)">进入</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div class="oj-pagination oj-list-pagination">
        <el-pagination
          v-model:current-page="store.contestList.page"
          v-model:page-size="store.contestList.pageSize"
          :total="store.contests.length"
          :page-sizes="[10, 20, 50, 100]"
          :pager-count="5"
          background
          layout="total, sizes, prev, pager, next"
          aria-label="比赛管理分页"
        />
      </div>
    </div>
  </div>
</template>
