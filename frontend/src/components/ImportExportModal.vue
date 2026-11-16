<script setup lang="ts">
import { ref, onMounted } from 'vue';
import type { Wallet, Snapshot, ImportedItem } from '../types';
import { api } from '../api/client';

const props = defineProps<{
  show: boolean;
  bookId: string;
  wallets: Wallet[];
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'imported'): void;
}>();

const activeTab = ref<'import' | 'export' | 'snapshots'>('import');
const importType = ref<'wechat' | 'alipay'>('wechat');
const parsedItems = ref<ImportedItem[]>([]);
const selectedWalletId = ref('');
const parsing = ref(false);
const committing = ref(false);
const snapshotLoading = ref(false);
const snapshots = ref<Snapshot[]>([]);
const statusMessage = ref<string | null>(null);
const errorMessage = ref<string | null>(null);

const loadSnapshots = async () => {
  try {
    snapshots.value = await api.listSnapshots();
  } catch {
    snapshots.value = [];
  }
};

const handleFileSelect = async (e: Event) => {
  const input = e.target as HTMLInputElement;
  if (!input.files || !input.files[0]) return;

  const file = input.files[0];
  parsing.value = true;
  statusMessage.value = null;
  errorMessage.value = null;

  try {
    let res;
    if (importType.value === 'wechat') {
      res = await api.importWeChat(file);
    } else {
      res = await api.importAlipay(file);
    }
    parsedItems.value = res.items || [];
    statusMessage.value = `成功解析 ${res.count} 条账单记录`;
    if (props.wallets.length > 0 && !selectedWalletId.value) {
      selectedWalletId.value = props.wallets[0].id;
    }
  } catch (err: any) {
    errorMessage.value = err.message || '账单解析失败';
  } finally {
    parsing.value = false;
  }
};

const handleCommitImport = async () => {
  if (parsedItems.value.length === 0 || !selectedWalletId.value) return;
  committing.value = true;
  errorMessage.value = null;

  try {
    const res = await api.commitImport(props.bookId, selectedWalletId.value, parsedItems.value);
    statusMessage.value = `已成功导入并入账 ${res.inserted} 条记录！`;
    parsedItems.value = [];
    emit('imported');
  } catch (err: any) {
    errorMessage.value = err.message || '导入入账失败';
  } finally {
    committing.value = false;
  }
};

const handleCreateSnapshot = async () => {
  snapshotLoading.value = true;
  errorMessage.value = null;
  try {
    const s = await api.createSnapshot();
    statusMessage.value = `热快照已成功创建: ${s.file_name}`;
    await loadSnapshots();
  } catch (err: any) {
    errorMessage.value = err.message || '创建快照失败';
  } finally {
    snapshotLoading.value = false;
  }
};

const handleRestoreSelect = async (e: Event) => {
  const input = e.target as HTMLInputElement;
  if (!input.files || !input.files[0]) return;
  const file = input.files[0];

  if (!confirm('恢复快照将会覆盖当前数据库，是否继续？')) {
    input.value = '';
    return;
  }

  snapshotLoading.value = true;
  errorMessage.value = null;
  try {
    await api.restoreSnapshot(file);
    alert('快照恢复成功！页面将自动刷新。');
    window.location.reload();
  } catch (err: any) {
    errorMessage.value = err.message || '快照恢复失败';
  } finally {
    snapshotLoading.value = false;
  }
};

const handleDeleteSnapshot = async (id: string) => {
  if (!confirm('确认删除该历史备份快照？')) return;
  try {
    await api.deleteSnapshot(id);
    await loadSnapshots();
  } catch (err: any) {
    errorMessage.value = err.message || '删除快照失败';
  }
};

onMounted(() => {
  if (props.wallets.length > 0) {
    selectedWalletId.value = props.wallets[0].id;
  }
  loadSnapshots();
});
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/30 backdrop-blur-[2px] transition-opacity"
  >
    <div
      class="w-full sm:max-w-2xl bg-[#FFFFFF] rounded-t-[20px] sm:rounded-[20px] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] border border-[#EDEEF1] flex flex-col max-h-[88vh] overflow-hidden"
    >
      <!-- Header with Tabs -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-[#EDEEF1]">
        <div class="flex items-center space-x-1 bg-[#F6F7F8] p-1 rounded-full border border-[#EDEEF1]">
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all cursor-pointer"
            :class="activeTab === 'import' ? 'bg-white text-[#171A20] shadow-xs' : 'text-[#5D6470]'"
            @click="activeTab = 'import'"
          >
            账单导入
          </button>
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all cursor-pointer"
            :class="activeTab === 'export' ? 'bg-white text-[#171A20] shadow-xs' : 'text-[#5D6470]'"
            @click="activeTab = 'export'"
          >
            账本导出
          </button>
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all cursor-pointer"
            :class="activeTab === 'snapshots' ? 'bg-white text-[#171A20] shadow-xs' : 'text-[#5D6470]'"
            @click="activeTab = 'snapshots'; loadSnapshots()"
          >
            热快照备份
          </button>
        </div>

        <button
          type="button"
          class="w-8 h-8 rounded-full flex items-center justify-center text-[#9AA0AB] hover:text-[#171A20] hover:bg-[#F6F7F8] cursor-pointer"
          @click="$emit('close')"
        >
          ✕
        </button>
      </div>

      <!-- Tab Content Area -->
      <div class="p-6 overflow-y-auto space-y-4">
        <!-- TAB 1: CSV IMPORT -->
        <div v-if="activeTab === 'import'" class="space-y-4">
          <div class="flex items-center space-x-3">
            <label class="text-xs font-medium text-[#5D6470]">账单类型:</label>
            <div class="flex items-center space-x-2">
              <button
                type="button"
                class="px-3 py-1 rounded-[8px] text-xs font-medium border transition-colors cursor-pointer"
                :class="
                  importType === 'wechat'
                    ? 'bg-[#EAF5EE] text-[#2E8B57] border-[#2E8B57]'
                    : 'bg-white text-[#5D6470] border-[#EDEEF1]'
                "
                @click="importType = 'wechat'; parsedItems = []"
              >
                微信支付账单 (UTF-8)
              </button>
              <button
                type="button"
                class="px-3 py-1 rounded-[8px] text-xs font-medium border transition-colors cursor-pointer"
                :class="
                  importType === 'alipay'
                    ? 'bg-[#E8F2FA] text-[#2577BA] border-[#2577BA]'
                    : 'bg-white text-[#5D6470] border-[#EDEEF1]'
                "
                @click="importType = 'alipay'; parsedItems = []"
              >
                支付宝账单 (GBK/UTF-8)
              </button>
            </div>
          </div>

          <!-- File Upload Zone -->
          <label
            class="flex flex-col items-center justify-center p-6 border-2 border-dashed border-[#EDEEF1] rounded-[14px] hover:border-[#0E8C7F] hover:bg-[#E4F3F1]/10 transition-colors cursor-pointer text-center"
          >
            <svg class="w-8 h-8 text-[#9AA0AB] mb-2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
              <line x1="16" y1="13" x2="8" y2="13" />
              <line x1="16" y1="17" x2="8" y2="17" />
              <polyline points="10 9 9 9 8 9" />
            </svg>
            <span class="text-xs font-medium text-[#171A20]">
              {{ parsing ? '正在解析账单文件...' : '点击或拖拽上传导出的账单 CSV 文件' }}
            </span>
            <span class="text-[11px] text-[#9AA0AB] mt-1">自动识别编码并跳过重复单号</span>
            <input type="file" accept=".csv" class="hidden" :disabled="parsing" @change="handleFileSelect" />
          </label>

          <!-- Parsed Items Preview -->
          <div v-if="parsedItems.length > 0" class="space-y-3 pt-2">
            <div class="flex items-center justify-between">
              <div class="flex items-center space-x-2">
                <span class="text-xs font-medium text-[#171A20]">待入账明细预览</span>
                <span class="text-xs text-[#0E8C7F] bg-[#E4F3F1] px-2 py-0.5 rounded-full font-mono">
                  {{ parsedItems.length }} 笔
                </span>
              </div>

              <!-- Wallet Mapping -->
              <div class="flex items-center space-x-2">
                <span class="text-xs text-[#5D6470]">归属账户:</span>
                <select
                  v-model="selectedWalletId"
                  class="h-8 px-2 text-xs bg-white rounded-[8px] border border-[#EDEEF1] outline-hidden text-[#171A20]"
                >
                  <option v-for="w in wallets" :key="w.id" :value="w.id">{{ w.name }}</option>
                </select>
              </div>
            </div>

            <div class="max-h-56 overflow-y-auto border border-[#EDEEF1] rounded-[11px] divide-y divide-[#EDEEF1]">
              <div
                v-for="(item, idx) in parsedItems"
                :key="idx"
                class="flex items-center justify-between p-2.5 text-xs bg-white hover:bg-[#F6F7F8]"
              >
                <div class="flex items-center space-x-2 truncate pr-2">
                  <span class="font-mono text-[#9AA0AB] text-[11px] shrink-0">{{ item.date }}</span>
                  <span class="font-medium text-[#171A20] truncate">{{ item.counterparty || item.product }}</span>
                  <span v-if="item.suggested_category_id" class="text-[10px] bg-[#E4F3F1] text-[#0E8C7F] px-1.5 py-0.5 rounded-full shrink-0">
                    {{ item.suggested_category_id }}
                  </span>
                </div>
                <div
                  class="font-bold tabular-nums tracking-tightest whitespace-nowrap"
                  :class="item.type === 'income' ? 'text-[#0E8C7F]' : 'text-[#E5654E]'"
                >
                  {{ item.type === 'income' ? '+' : '-' }}¥{{ (item.amount / 100).toFixed(2) }}
                </div>
              </div>
            </div>

            <!-- Commit Button -->
            <button
              type="button"
              class="w-full h-11 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white font-medium text-xs rounded-full transition-all flex items-center justify-center space-x-2 cursor-pointer disabled:opacity-50"
              :disabled="committing"
              @click="handleCommitImport"
            >
              <span v-if="committing">正在入账写入...</span>
              <span v-else>确认提交入账 (共 {{ parsedItems.length }} 笔)</span>
            </button>
          </div>
        </div>

        <!-- TAB 2: FULL LEDGER EXPORT -->
        <div v-else-if="activeTab === 'export'" class="space-y-4">
          <div class="p-5 bg-[#F6F7F8] rounded-[14px] border border-[#EDEEF1] space-y-3">
            <h4 class="text-sm font-semibold text-[#171A20]">导出当前账本全量明细</h4>
            <p class="text-xs text-[#5D6470] leading-relaxed">
              导出格式为标准 UTF-8 BOM CSV 文件，完美兼容微软 Excel、WPS Office 和各类财务表格软件，无任何中文乱码问题。
            </p>
            <a
              :href="api.getExportUrl(bookId)"
              download="ledger_export.csv"
              class="inline-flex items-center space-x-2 px-5 py-2.5 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white text-xs font-medium rounded-full shadow-xs transition-colors"
            >
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7 10 12 15 17 10" />
                <line x1="12" y1="15" x2="12" y2="3" />
              </svg>
              <span>立即下载账本 CSV</span>
            </a>
          </div>
        </div>

        <!-- TAB 3: HOT SNAPSHOTS & BACKUP -->
        <div v-else-if="activeTab === 'snapshots'" class="space-y-4">
          <div class="flex items-center justify-between">
            <div>
              <h4 class="text-sm font-semibold text-[#171A20]">在线热备份快照</h4>
              <p class="text-[11px] text-[#9AA0AB]">基于 SQLite VACUUM INTO 引擎，无锁热备份整库与票据附件</p>
            </div>
            <div class="flex items-center space-x-2">
              <button
                type="button"
                class="px-3 py-1.5 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white text-xs font-medium rounded-full transition-colors cursor-pointer disabled:opacity-50"
                :disabled="snapshotLoading"
                @click="handleCreateSnapshot"
              >
                + 创建新快照
              </button>
              <label
                class="px-3 py-1.5 bg-white border border-[#EDEEF1] hover:bg-[#F6F7F8] text-[#171A20] text-xs font-medium rounded-full transition-colors cursor-pointer"
              >
                <span>恢复快照</span>
                <input type="file" accept=".ugledger,.tar.gz" class="hidden" @change="handleRestoreSelect" />
              </label>
            </div>
          </div>

          <!-- Snapshots List -->
          <div v-if="snapshots.length === 0" class="h-32 flex items-center justify-center text-xs text-[#9AA0AB]">
            暂无历史快照记录
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="snap in snapshots"
              :key="snap.id"
              class="flex items-center justify-between p-3 rounded-[11px] bg-[#F6F7F8] border border-[#EDEEF1]"
            >
              <div class="space-y-0.5">
                <div class="text-xs font-mono font-medium text-[#171A20]">{{ snap.file_name }}</div>
                <div class="text-[10px] font-mono text-[#9AA0AB]">
                  {{ (snap.file_size / 1024).toFixed(1) }} KB · SHA256: {{ snap.sha256.slice(0, 8) }}...
                </div>
              </div>
              <div class="flex items-center space-x-2">
                <a
                  :href="api.getSnapshotDownloadUrl(snap.id)"
                  download
                  class="px-2.5 py-1 bg-white border border-[#EDEEF1] text-[11px] text-[#0E8C7F] rounded-[7px] hover:bg-[#E4F3F1] transition-colors"
                >
                  下载
                </a>
                <button
                  type="button"
                  class="px-2.5 py-1 text-[11px] text-[#E5654E] hover:underline cursor-pointer"
                  @click="handleDeleteSnapshot(snap.id)"
                >
                  删除
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Status & Error Notices -->
        <div v-if="statusMessage" class="text-xs text-[#0E8C7F] bg-[#E4F3F1] p-2.5 rounded-[11px]">
          {{ statusMessage }}
        </div>
        <div v-if="errorMessage" class="text-xs text-[#E5654E] bg-[#FDF1EE] p-2.5 rounded-[11px]">
          {{ errorMessage }}
        </div>
      </div>
    </div>
  </div>
</template>
