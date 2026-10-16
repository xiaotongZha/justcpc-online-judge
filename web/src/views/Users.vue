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
  Object.assign(form, {
    id: item.id,
    username: item.username,
    password: item.password,
    role: item.role,
  })
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
      <div class="oj-heading-copy">
        <h2>{{ store.account?.role === 'admin' ? '账户管理' : '我的账户' }}</h2>
        <p>
          {{
            store.account?.role === 'admin' ? '管理试题组的平台访问账户' : '查看当前登录账户信息'
          }}
        </p>
      </div>
      <el-button v-if="store.account?.role === 'admin'" type="primary" @click="openCreate"
        ><el-icon><Plus /></el-icon>新建账户</el-button
      >
    </div>
    <div class="oj-card">
      <div class="oj-card-body oj-account-card">
        <el-avatar :size="44">{{ store.account?.username.slice(0, 1).toUpperCase() }}</el-avatar>
        <div>
          <span class="oj-table-id" style="margin: 0 0 3px">当前登录账户</span
          ><strong>{{ store.account?.username }}</strong>
        </div>
        <el-tag size="small" :type="store.account?.role === 'admin' ? 'primary' : 'info'">{{
          store.account?.role === 'admin' ? '管理员' : '普通用户'
        }}</el-tag>
      </div>
    </div>
    <div v-if="store.account?.role === 'admin'" class="oj-card">
      <div class="oj-card-header">
        <h3>平台账户</h3>
        <span class="small muted">{{ store.accounts.length }} 个账户</span>
      </div>
      <div class="oj-table-wrap">
        <el-table :data="store.accounts">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="username" label="用户名" min-width="180" />
          <el-table-column label="密码" min-width="180"
            ><template #default="{ row }"
              ><span v-if="row.password" class="mono muted">{{ row.password }}</span
              ><span v-else class="muted">旧账户，需重设</span></template
            ></el-table-column
          >
          <el-table-column label="身份" width="140"
            ><template #default="{ row }"
              ><el-tag :type="row.role === 'admin' ? 'primary' : 'info'" size="small">{{
                row.role === 'admin' ? '管理员' : '普通用户'
              }}</el-tag></template
            ></el-table-column
          >
          <el-table-column label="操作" width="100" fixed="right"
            ><template #default="{ row }"
              ><el-button size="small" text type="primary" @click="openEdit(row)"
                >编辑</el-button
              ></template
            ></el-table-column
          >
        </el-table>
      </div>
    </div>
    <p class="oj-help">比赛选手在比赛数据包中管理，其提交与分数可在比赛和提交记录中查看。</p>
    <el-dialog
      v-model="dialogOpen"
      :title="form.id ? '编辑账户' : '新建账户'"
      width="440px"
      align-center
    >
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item label="用户名" required
          ><el-input v-model.trim="form.username" autocomplete="off"
        /></el-form-item>
        <el-form-item label="密码" required
          ><el-input
            v-model="form.password"
            type="password"
            show-password
            autocomplete="new-password"
        /></el-form-item>
        <el-form-item label="身份"
          ><el-select v-model="form.role" style="width: 100%"
            ><el-option label="普通用户" value="user" /><el-option
              label="管理员"
              value="admin" /></el-select
        ></el-form-item>
      </el-form>
      <template #footer
        ><el-button @click="dialogOpen = false">取消</el-button
        ><el-button type="primary" @click="save">{{
          form.id ? '保存账户' : '创建账户'
        }}</el-button></template
      >
    </el-dialog>
  </div>
</template>
