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
      <h2>比赛</h2>
      <el-button type="primary" @click="navigate('contest-create')">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建比赛
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-table-wrap">
        <el-table :data="store.contests" stripe size="small" v-loading="store.loading">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column prop="name" label="名称" min-width="200" show-overflow-tooltip />
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
          <el-table-column label="操作" width="90" align="center">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openContest(row)">进入</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>
