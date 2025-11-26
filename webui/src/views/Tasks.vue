<template>
  <section class="section-grid">
    <div class="card form-card">
      <div class="card-header">
        <div>
          <h3>提交任务</h3>
          <p class="muted small-text">填写任务信息后提交到集群，支持 MinIO 上传与输出目录挂载。</p>
        </div>
        <button class="btn-ghost btn-small" type="button" @click="resetForm">重置</button>
      </div>
      <form class="task-form" @submit.prevent="submitTask">
        <div class="two-col">
          <div>
            <label>任务名称</label>
            <input v-model="form.name" required placeholder="图像分类" />
          </div>
          <div>
            <label>训练镜像</label>
            <input v-model="form.image" required placeholder="gpuconductor/train:latest" />
          </div>
        </div>
        <div>
          <label>启动命令</label>
          <input v-model="form.command" required placeholder="python main.py" />
          <p class="muted small-text">如果填写了“训练脚本路径”，将优先执行脚本。</p>
        </div>
        <div class="two-col">
          <div>
            <label>数据集 MinIO 路径</label>
            <input v-model="form.dataset_path" placeholder="s3://bucket/dataset" />
          </div>
          <div>
            <label>模型输出(宿主机)</label>
            <input v-model="form.model_output_path" placeholder="/data/output/run-001" />
          </div>
        </div>
        <div class="two-col">
          <div>
            <label>模型输出(容器内)</label>
            <input v-model="form.model_output_container" placeholder="/workspace/output" />
          </div>
          <div></div>
        </div>
        <div>
          <label>训练脚本路径</label>
          <input v-model="form.script_path" placeholder="scripts/train.sh" />
        </div>
        <div>
          <label>代码仓库地址</label>
          <input v-model="form.code_repo" placeholder="https://git.example.com/group/project.git" />
        </div>
        <div class="two-col">
          <div>
            <label>迭代次数</label>
            <input type="number" min="1" v-model.number="form.iterations" />
          </div>
          <div>
            <label>GPU 数量</label>
            <input type="number" min="1" :max="maxGpuCount" v-model.number="form.gpu_count" />
            <p class="muted small-text">最大可选 {{ maxGpuCount }} 张 GPU</p>
          </div>
        </div>
        <div class="two-col">
          <div>
            <label>指定节点</label>
            <select v-model="form.node_id">
              <option value="">自动分配</option>
              <option v-for="node in nodes" :key="node.id" :value="node.id">
                {{ node.name || node.id }} ({{ node.status }})
              </option>
            </select>
          </div>
          <div>
            <label>最大运行时间 (分钟)</label>
            <input type="number" min="1" v-model.number="form.max_duration" placeholder="60" />
          </div>
        </div>
        <button class="btn-primary" type="submit" :disabled="submitting">
          {{ submitting ? '提交中...' : '提交' }}
        </button>
        <p class="error" v-if="error">{{ error }}</p>
      </form>
    </div>

    <div class="card task-card">
      <div class="card-header">
        <div>
          <h3>任务列表</h3>
          <p class="muted small-text">状态实时刷新，可查看详情、日志并取消任务。</p>
        </div>
        <div class="task-toolbar">
          <select v-model="statusFilter" @change="loadTasks">
            <option value="">全部状态</option>
            <option value="running">运行中</option>
            <option value="pending">等待中</option>
            <option value="completed">已完成</option>
            <option value="failed">失败</option>
            <option value="cancelled">已取消</option>
          </select>
          <button class="btn-ghost btn-small" type="button" @click="loadTasks" :disabled="loadingTasks">
            {{ loadingTasks ? '刷新中...' : '刷新' }}
          </button>
        </div>
      </div>

      <div class="task-stats">
        <div class="stat-chip">
          <span class="muted">总任务</span>
          <strong>{{ taskStats.total }}</strong>
        </div>
        <div class="stat-chip">
          <span class="muted">运行中</span>
          <strong>{{ taskStats.running }}</strong>
        </div>
        <div class="stat-chip">
          <span class="muted">等待中</span>
          <strong>{{ taskStats.pending }}</strong>
        </div>
        <div class="stat-chip">
          <span class="muted">失败/取消</span>
          <strong>{{ taskStats.unhealthy }}</strong>
        </div>
      </div>

      <div class="table-wrapper">
        <table class="table">
          <thead>
            <tr>
              <th>名称</th>
              <th>状态</th>
              <th>运行时长</th>
              <th>GPU / 节点</th>
              <th>提交时间</th>
              <th>耗时</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in tasks" :key="task.id">
              <td>
                <div class="task-name">{{ task.name }}</div>
                <div class="muted small-text">ID: {{ shortId(task.id) }}</div>
              </td>
              <td>
                <span :class="['badge', statusClass(task.status)]">{{ statusText(task.status) }}</span>
              </td>
              <td>
                <div class="progress">
                  <div
                    class="bar"
                    :class="{ overrun: isOverrun(task) }"
                    :style="{ width: runtimePercent(task) + '%' }"
                  ></div>
                </div>
                <div class="muted small-text">{{ runtimeText(task) }}</div>
              </td>
              <td>
                <div>GPU: {{ task.gpu_count || 0 }}</div>
                <div class="muted small-text">节点: {{ task.assigned_node_id || '-' }}</div>
              </td>
              <td>{{ formatTime(task.created_at) }}</td>
              <td>{{ durationText(task) }}</td>
              <td class="actions-cell">
                <div class="action-buttons">
                  <button class="btn-ghost btn-small" type="button" @click="selectTask(task)">详情</button>
                  <button class="btn-ghost btn-small" type="button" @click="openLogs(task)">日志</button>
                  <button
                    class="btn-primary btn-small"
                    type="button"
                    @click="cancelTask(task)"
                    :disabled="!canCancel(task)"
                  >
                    取消
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!tasks.length">
              <td colspan="6" class="muted">暂无任务</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>

  <div class="drawer-backdrop" v-if="detailOpen">
    <div class="drawer">
      <div class="drawer-header">
        <div>
          <h4>{{ selectedTask?.name }}</h4>
          <p class="muted small-text">任务 ID: {{ selectedTask?.id }}</p>
        </div>
        <button class="btn-ghost btn-small" @click="detailOpen = false">关闭</button>
      </div>
      <div class="detail-grid" v-if="selectedTask">
        <div class="detail-item">
          <span class="muted">状态</span>
          <div><span :class="['badge', statusClass(selectedTask.status)]">{{ statusText(selectedTask.status) }}</span></div>
        </div>
        <div class="detail-item">
          <span class="muted">容器</span>
          <div>{{ selectedTask.container_id || '-' }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">节点</span>
          <div>{{ selectedTask.assigned_node_id || '-' }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">运行时长</span>
          <div>{{ runtimeText(selectedTask) }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">退出码</span>
          <div>{{ selectedTask.exit_code ?? '-' }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">耗时</span>
          <div>{{ durationText(selectedTask) }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">创建时间</span>
          <div>{{ formatTime(selectedTask.created_at) }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">开始时间</span>
          <div>{{ formatTime(selectedTask.started_at) }}</div>
        </div>
        <div class="detail-item">
          <span class="muted">完成时间</span>
          <div>{{ formatTime(selectedTask.completed_at) }}</div>
        </div>
      </div>
      <div class="detail-block" v-if="selectedTask">
        <h5>运行配置</h5>
        <div class="detail-row"><span class="muted">镜像</span><span>{{ selectedTask.image }}</span></div>
        <div class="detail-row"><span class="muted">命令</span><span class="code-text">{{ selectedTask.command }}</span></div>
        <div class="detail-row"><span class="muted">数据集</span><span>{{ selectedTask.dataset_path || '-' }}</span></div>
        <div class="detail-row"><span class="muted">输出</span><span>{{ selectedTask.model_output_path || '-' }}</span></div>
        <div class="detail-row"><span class="muted">输出(容器)</span><span>{{ selectedTask.model_output_container || '-' }}</span></div>
        <div class="detail-row"><span class="muted">MinIO</span><span>{{ selectedTask.minio_endpoint || '-' }}</span></div>
        <div class="detail-row"><span class="muted">Bucket</span><span>{{ selectedTask.minio_bucket || '-' }}</span></div>
        <div class="detail-row"><span class="muted">脚本</span><span>{{ selectedTask.script_path || '-' }}</span></div>
        <div class="detail-row"><span class="muted">代码仓库</span><span class="code-text">{{ selectedTask.code_repo || '-' }}</span></div>
        <div class="detail-row"><span class="muted">GPU 数量</span><span>{{ selectedTask.gpu_count }}</span></div>
        <div class="detail-row"><span class="muted">迭代次数</span><span>{{ selectedTask.iterations || '-' }}</span></div>
        <div class="detail-row"><span class="muted">错误信息</span><span>{{ selectedTask.error_message || '-' }}</span></div>
      </div>
    </div>
  </div>

  <div class="modal-backdrop" v-if="logsState.open">
    <div class="modal">
      <div class="modal-header">
        <div>
          <h4>任务日志</h4>
          <p class="muted small-text">{{ logsState.task?.name }} · 容器 {{ shortId(logsState.task?.container_id) }}</p>
        </div>
        <div class="action-buttons">
          <button class="btn-ghost btn-small" type="button" @click="refreshLogs" :disabled="logsState.loading">
            {{ logsState.loading ? '加载中...' : '刷新' }}
          </button>
          <button class="btn-primary btn-small" type="button" @click="closeLogs">关闭</button>
        </div>
      </div>
      <div class="log-box">
        <p class="muted" v-if="logsState.loading">日志加载中...</p>
        <p class="error" v-else-if="logsState.error">{{ logsState.error }}</p>
        <p class="muted" v-else-if="!logsText">暂无日志</p>
        <pre v-else>{{ logsText }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { useAuthStore } from '../store'

const auth = useAuthStore()
const error = ref('')
const submitting = ref(false)
const loadingTasks = ref(false)
const statusFilter = ref('')
const poller = ref(null)
const nodes = computed(() => auth.nodes || [])

const createFormDefaults = () => ({
  name: '',
  image: 'gpuconductor/train:latest',
  command: 'python main.py',
  dataset_path: '',
  model_output_path: '',
  model_output_container: '/workspace/output',
  script_path: '',
  code_repo: '',
  node_id: '',
  iterations: 1,
  gpu_count: 1,
  max_duration: 60,
  environment: {}
})

const form = reactive(createFormDefaults())
const selectedTask = ref(null)
const detailOpen = ref(false)

const logsState = reactive({
  open: false,
  loading: false,
  logs: [],
  error: '',
  task: null
})

const tasks = computed(() => auth.tasks?.tasks || [])

const taskStats = computed(() => {
  const list = tasks.value
  return {
    total: list.length,
    running: list.filter((t) => t.status === 'running').length,
    pending: list.filter((t) => t.status === 'pending').length,
    unhealthy: list.filter((t) => t.status === 'failed' || t.status === 'cancelled').length
  }
})

const submitTask = async () => {
  error.value = ''
  submitting.value = true
  try {
    await auth.createTask(form)
    resetForm()
    await loadTasks()
  } catch (err) {
    error.value = err.response?.data?.error || '提交失败'
  } finally {
    submitting.value = false
  }
}

const resetForm = () => {
  Object.assign(form, createFormDefaults())
}

const loadTasks = async () => {
  loadingTasks.value = true
  try {
    const params = { page: 1, pageSize: 50 }
    if (statusFilter.value) {
      params.status = statusFilter.value
    }
    await auth.fetchTasks(params)
  } catch (err) {
    error.value = err.response?.data?.error || '获取任务失败'
  } finally {
    loadingTasks.value = false
  }
}

const cancelTask = async (task) => {
  try {
    await auth.cancelTask(task.id)
    await loadTasks()
  } catch (err) {
    error.value = err.response?.data?.error || '取消任务失败'
  }
}

const canCancel = (task) => task.status === 'running' || task.status === 'pending'

const maxGpuCount = computed(() => {
  const list = nodes.value || []
  if (!list.length) return form.gpu_count || 1
  if (form.node_id) {
    const node = list.find((n) => n.id === form.node_id)
    if (node) {
      const count = Array.isArray(node.gpus) ? node.gpus.length : 0
      return count || 1
    }
  }
  const max = Math.max(...list.map((n) => (Array.isArray(n.gpus) ? n.gpus.length : 0)))
  return max || 1
})

const statusClass = (status) => (status ? `badge-${status}` : '')
const statusText = (status) =>
  (
    {
      running: '运行中',
      pending: '等待中',
      completed: '已完成',
      failed: '失败',
      cancelled: '已取消'
    }[status] || status || '未知'
  )

const formatTime = (value) => (value ? new Date(value).toLocaleString() : '-')
const shortId = (value) => (value ? String(value).slice(0, 12) : '-')

const durationText = (task) => {
  if (!task) return '-'
  const start = task.started_at || task.created_at
  if (!start) return '-'
  const end = task.completed_at || new Date().toISOString()
  const diffMs = new Date(end) - new Date(start)
  if (Number.isNaN(diffMs) || diffMs < 0) return '-'
  const mins = Math.floor(diffMs / 60000)
  const hours = Math.floor(mins / 60)
  const remain = mins % 60
  if (hours > 0) {
    return `${hours} 小时 ${remain} 分钟`
  }
  return `${mins || 0} 分钟`
}

const runtimeMs = (task) => {
  if (!task) return null
  const start = task.started_at || task.created_at
  if (!start) return null
  const end = task.completed_at || new Date().toISOString()
  const diff = new Date(end) - new Date(start)
  return Number.isNaN(diff) || diff < 0 ? null : diff
}

const runtimeText = (task) => {
  const elapsed = runtimeMs(task)
  if (elapsed == null) return '-'
  const mins = Math.floor(elapsed / 60000)
  const secs = Math.floor((elapsed % 60000) / 1000)
  const max = task?.max_duration || 0
  const base = mins > 0 ? `${mins}分${secs.toString().padStart(2, '0')}秒` : `${secs}秒`
  if (max > 0) {
    const over = mins - max
    if (over > 0) return `${base} · 超时 ${over}分`
    return `${base} / ${max}分`
  }
  return base
}

const runtimePercent = (task) => {
  const elapsed = runtimeMs(task)
  const max = task?.max_duration || 0
  if (elapsed == null || max <= 0) return 0
  const pct = (elapsed / (max * 60000)) * 100
  return Math.min(100, Math.max(0, Math.round(pct)))
}

const isOverrun = (task) => {
  const elapsed = runtimeMs(task)
  const max = task?.max_duration || 0
  return max > 0 && elapsed != null && elapsed > max * 60000
}

const selectTask = (task) => {
  selectedTask.value = task
  detailOpen.value = true
}

const logsText = computed(() =>
  logsState.logs
    .map((item) => {
      const time = item.timestamp ? new Date(item.timestamp).toLocaleString() : ''
      return time ? `[${time}] ${item.content}` : item.content
    })
    .join('\n')
)

const fetchLogs = async (task) => {
  logsState.loading = true
  logsState.error = ''
  try {
    logsState.logs = await api.taskLogs(task.id)
  } catch (err) {
    logsState.error = err.response?.data?.error || '获取日志失败'
  } finally {
    logsState.loading = false
  }
}

const openLogs = async (task) => {
  logsState.task = task
  logsState.open = true
  await fetchLogs(task)
}

const refreshLogs = async () => {
  if (logsState.task) {
    await fetchLogs(logsState.task)
  }
}

const closeLogs = () => {
  logsState.open = false
  logsState.logs = []
  logsState.task = null
}

watch(statusFilter, loadTasks)

watch(
  () => [form.node_id, nodes.value],
  () => {
    const max = maxGpuCount.value
    if (form.gpu_count > max) {
      form.gpu_count = max
    }
    if (form.gpu_count < 1) {
      form.gpu_count = 1
    }
  },
  { deep: true }
)

onMounted(async () => {
  await loadTasks()
  try {
    await auth.fetchNodes()
  } catch (err) {
    console.error('获取节点失败', err)
  }
  poller.value = setInterval(loadTasks, 5000)
})

onUnmounted(() => {
  if (poller.value) {
    clearInterval(poller.value)
  }
})
</script>

<style scoped>
.task-form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.9rem;
}

.task-toolbar {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.task-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.75rem;
  margin-bottom: 0.6rem;
}

.stat-chip {
  padding: 0.65rem;
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.35);
}

.task-name {
  font-weight: 600;
  font-size: 1.05rem;
}

.small-text {
  font-size: 0.9rem;
}

.progress {
  width: 100%;
  height: 6px;
  background: rgba(148, 163, 184, 0.2);
  border-radius: 999px;
  overflow: hidden;
}

.progress .bar {
  height: 100%;
  background: var(--accent);
  width: 0;
  transition: width 0.2s ease;
}

.progress .bar.overrun {
  background: #ef4444;
}

.input-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.select-row {
  margin-top: 0.5rem;
}

.actions-cell {
  width: 200px;
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.drawer-backdrop,
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 20;
  padding: 1rem;
}

.drawer {
  width: min(780px, 100%);
  background: linear-gradient(135deg, rgba(20, 31, 57, 0.95), rgba(16, 24, 48, 0.9));
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 18px;
  padding: 1.5rem;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(10px);
}

.drawer-header,
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 1rem;
}

.detail-item {
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 12px;
  padding: 0.9rem;
  background: rgba(30, 41, 75, 0.55);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.detail-block {
  margin-top: 1.2rem;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 14px;
  padding: 1rem;
}

.detail-block h5 {
  margin: 0 0 0.6rem;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.65rem 0;
  border-bottom: 1px dashed rgba(148, 163, 184, 0.25);
  align-items: center;
}

.detail-row:last-child {
  border-bottom: none;
}

.code-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  word-break: break-all;
}

.modal {
  width: min(900px, 100%);
  background: linear-gradient(135deg, rgba(20, 31, 57, 0.95), rgba(16, 24, 48, 0.9));
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  padding: 1.25rem;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(10px);
}

.log-box {
  background: rgba(15, 23, 42, 0.45);
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 12px;
  min-height: 320px;
  max-height: 480px;
  overflow-y: auto;
  padding: 1rem;
}

pre {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

.error {
  color: #f87171;
}

@media (max-width: 960px) {
  .two-col {
    grid-template-columns: 1fr;
  }

  .actions-cell {
    width: auto;
  }
}
</style>
