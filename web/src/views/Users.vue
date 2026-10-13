<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { store, refresh, type AdminAccount } from '../store'

const dialogOpen = ref(false)
const form = reactive({ id: 0, username: '', password: '123456', role: 'user' })

function openCreate() {
  Object.assign(form, { id: 0, username: '', password: '123456', role: 'user' })
  dialogOpen.value = true
}
function openEdit(item: AdminAccount) {
  Object.assign(form, { id: item.id, username: item.username, password: item.password, role: item.role })
  dialogOpen.value = true
}
async function save() {
  try {
    if (form.id > 0) {
      await api(`/accounts/${form.id}`, {
        method: 'PUT',
        body: JSON.stringify({ username: form.username, password: form.password, role: form.role }),
      })
      ElMessage.success('账户已保存')
    } else {
      await api('/accounts', {
        method: 'POST',
        body: JSON.stringify({ username: form.username, password: form.password, role: form.role }),
      })
      ElMessage.success('账户已创建')
    }
    dialogOpen.value = false
    await refresh()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <h2>用户</h2>
    </div>

    <div class="oj-card">
      <div class="oj-card-body row">
        <el-avatar :size="40" class="el-icon">
          <el-icon><UserFilled /></el-icon>
        </el-avatar>
        <div>
          <div style="font-weight: 600">{{ store.account?.username }}</div>
          <div class="muted small">{{ store.account?.role === 'admin' ? '管理员' : '普通用户' }}</div>
        </div>
        <el-tag size="small" :type="store.account?.role === 'admin' ? 'primary' : 'info'" style="margin-left: 4px">
          {{ store.account?.role === 'admin' ? '管理员' : '普通用户' }}
        </el-tag>
      </div>
    </div>

    <template v-if="store.account?.role === 'admin'">
      <div class="oj-card" style="margin-top: 16px">
        <div class="oj-card-header">
          <h3>平台账户</h3>
          <el-button type="primary" size="small" @click="openCreate">
            <el-icon style="margin-right: 4px"><Plus /></el-icon>新建账户
          </el-button>
        </div>
        <div class="oj-table-wrap">
          <el-table :data="store.accounts" stripe size="small">
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="username" label="用户名" min-width="140" />
            <el-table-column label="密码" min-width="160">
              <template #default="{ row }">
                <span v-if="row.password" class="mono muted">{{ row.password }}</span>
                <span v-else class="muted">旧账户，需重设</span>
              </template>
            </el-table-column>
            <el-table-column label="身份" width="120">
              <template #default="{ row }">
                <el-tag :type="row.role === 'admin' ? 'primary' : 'info'" size="small">
                  {{ row.role === 'admin' ? '管理员' : '普通用户' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="110">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </template>

    <div class="oj-card" style="margin-top: 16px">
      <div class="oj-card-body">
        <p style="margin: 0" class="muted">
          比赛虚拟用户只在比赛数据包中管理，不可登录；其提交与分数在比赛、题目和提交页面查看。
        </p>
      </div>
    </div>

    <el-dialog v-model="dialogOpen" :title="form.id ? '编辑账户' : '新建账户'" width="420px" align-center>
      <el-form label-width="60px" @submit.prevent="save">
        <el-form-item label="用户名" required>
          <el-input v-model.trim="form.username" autocomplete="off" />
        </el-form-item>
        <el-form-item label="密码" required>
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="身份">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogOpen = false">取消</el-button>
        <el-button type="primary" @click="save">{{ form.id ? '保存账户' : '创建账户' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>
