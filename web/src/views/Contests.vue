<script setup lang="ts">
import { store, navigate } from '../store'
import type { Contest } from '../store'

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
        <el-table :data="store.contests" v-loading="store.loading" empty-text="尚未创建比赛">
          <el-table-column label="比赛" min-width="260">
            <template #default="{ row }"
              ><button class="oj-title-link" @click="openContest(row)">{{ row.name }}</button
              ><span class="oj-table-id">#{{ row.id }}</span></template
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
      <div class="oj-card-footer">共 {{ store.contests.length }} 场比赛</div>
    </div>
  </div>
</template>
