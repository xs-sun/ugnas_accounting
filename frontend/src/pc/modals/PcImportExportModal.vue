<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { downloadWithAuth } from '@/pc/utils/adminAdapter';
import { formatMoney } from '@/utils/format';
import type { ImportedItem } from '@/types';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  close: [];
  imported: [count: number];
}>();

const store = useAppStore();

const activeTab = ref<'import' | 'export'>('import');
const platform = ref<'wechat' | 'alipay'>('wechat');
const targetWalletId = ref('');
const parsing = ref(false);
const committing = ref(false);
const exporting = ref(false);
const selectedFileName = ref('');
const parsedItems = ref<ImportedItem[]>([]);
const errorMsg = ref('');

const walletOptions = computed<DropdownOption[]>(() =>
  store.activeWallets.map((w) => ({
    value: w.id,
    label: w.name,
    dot: w.color || '#DE8A1E',
  }))
);

const importSummary = computed(() => {
  let expense = 0;
  let income = 0;
  for (const item of parsedItems.value) {
    if (item.type === 'income') income += item.amount || 0;
    else expense += item.amount || 0;
  }
  return {
    count: parsedItems.value.length,
    expense,
    income,
  };
});

const previewRows = computed(() => parsedItems.value.slice(0, 20));

watch(
  () => props.open,
  (val) => {
    if (val) {
      errorMsg.value = '';
      if (!targetWalletId.value || !store.activeWallets.some((w) => w.id === targetWalletId.value)) {
        targetWalletId.value = store.activeWallets[0]?.id || '';
      }
    }
  }
);

function resetImportState() {
  selectedFileName.value = '';
  parsedItems.value = [];
  errorMsg.value = '';
}

async function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  errorMsg.value = '';
  selectedFileName.value = file.name;
  parsing.value = true;
  try {
    const res =
      platform.value === 'wechat'
        ? await api.importWeChat(file)
        : await api.importAlipay(file);
    parsedItems.value = res.items || [];
    if (!parsedItems.value.length) {
      errorMsg.value = '文件中未解析到有效收支记录，请检查账单平台选择或文件格式';
    } else {
      store.showToast(`成功解析 ${parsedItems.value.length} 条流水记录`, 'success');
    }
  } catch (err: any) {
    parsedItems.value = [];
    errorMsg.value = err?.message || '解析账单文件失败，请确认 CSV 格式正确';
    store.showToast(errorMsg.value, 'error');
  } finally {
    parsing.value = false;
    input.value = '';
  }
}

async function handleCommitImport() {
  if (committing.value || !parsedItems.value.length) return;
  if (!store.activeBookId) {
    errorMsg.value = '请先选择账本';
    return;
  }
  const wid = targetWalletId.value || store.activeWallets[0]?.id || '';
  if (!wid) {
    errorMsg.value = '当前账本暂无可用账户，请先创建资产账户';
    return;
  }

  committing.value = true;
  errorMsg.value = '';
  try {
    const res = await api.commitImport(store.activeBookId, wid, parsedItems.value);
    await store.refreshBookData(store.activeBookId);
    const count = res.inserted ?? parsedItems.value.length;
    store.showToast(`已成功导入 ${count} 条账单记录`, 'success');
    resetImportState();
    emit('imported', count);
    emit('close');
  } catch (err: any) {
    errorMsg.value = err?.message || '入库失败，请重试';
    store.showToast(errorMsg.value, 'error');
  } finally {
    committing.value = false;
  }
}

async function handleExportCsv() {
  if (exporting.value) return;
  exporting.value = true;
  errorMsg.value = '';
  try {
    const bookName = (store.activeBook?.name || '七猫账本').replace(/[\\/:*?"<>|]/g, '_');
    const monthStr = `${store.currentYear}-${String(store.currentMonth).padStart(2, '0')}`;
    const url = api.getExportUrl(store.activeBookId || undefined);
    await downloadWithAuth(url, `${bookName}_${monthStr}.csv`);
    store.showToast('CSV 账单已开始下载', 'success');
  } catch (err: any) {
    errorMsg.value = err?.message || '导出失败，请稍后重试';
    store.showToast(errorMsg.value, 'error');
  } finally {
    exporting.value = false;
  }
}

function handleClose() {
  if (parsing.value || committing.value || exporting.value) return;
  emit('close');
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[60] flex items-center justify-center p-4">
      <div
        class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
        @click="handleClose"
      />

      <div
        class="relative w-full max-w-[680px] max-h-[88vh] bg-white rounded-[26px] border border-[#EAECEF] shadow-2xl flex flex-col overflow-hidden"
      >
        <!-- Header -->
        <div class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between gap-3 shrink-0">
          <div class="inline-flex items-center p-1 rounded-full bg-[#F1F3F6]">
            <button
              type="button"
              class="px-4 py-1.5 rounded-full text-xs font-semibold transition"
              :class="
                activeTab === 'import'
                  ? 'bg-white text-[#18181B] shadow-xs'
                  : 'text-[#64748B] hover:text-[#18181B]'
              "
              @click="activeTab = 'import'"
            >
              导入微信 / 支付宝账单
            </button>
            <button
              type="button"
              class="px-4 py-1.5 rounded-full text-xs font-semibold transition"
              :class="
                activeTab === 'export'
                  ? 'bg-white text-[#DE8A1E] shadow-xs'
                  : 'text-[#64748B] hover:text-[#18181B]'
              "
              @click="activeTab = 'export'"
            >
              导出当前账本 CSV
            </button>
          </div>

          <button
            type="button"
            aria-label="关闭弹窗"
            class="w-8 h-8 rounded-full hover:bg-[#F4F5F8] flex items-center justify-center text-[#64748B] hover:text-[#18181B] transition"
            @click="handleClose"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Body -->
        <div class="p-5 overflow-y-auto thin-scroll space-y-4 bg-[#F8F9FB] flex-1 text-xs">
          <div
            v-if="errorMsg"
            class="px-3.5 py-2.5 rounded-xl bg-[#FDF0EE] border border-[#F5C4BC] text-[#D6553E] font-medium"
          >
            {{ errorMsg }}
          </div>

          <!-- 导入面板 -->
          <template v-if="activeTab === 'import'">
            <div class="bg-white border border-[#EAECEF] rounded-2xl p-4 space-y-4">
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label class="block text-[11px] font-semibold text-[#64748B] mb-1.5">账单来源平台</label>
                  <div class="grid grid-cols-2 p-1 rounded-xl bg-[#F4F5F8] font-semibold">
                    <button
                      type="button"
                      class="py-1.5 rounded-lg transition"
                      :class="
                        platform === 'wechat'
                          ? 'bg-white text-[#3E8E63] shadow-xs'
                          : 'text-[#64748B]'
                      "
                      @click="platform = 'wechat'; resetImportState()"
                    >
                      微信支付 CSV
                    </button>
                    <button
                      type="button"
                      class="py-1.5 rounded-lg transition"
                      :class="
                        platform === 'alipay'
                          ? 'bg-white text-[#2F78A8] shadow-xs'
                          : 'text-[#64748B]'
                      "
                      @click="platform = 'alipay'; resetImportState()"
                    >
                      支付宝 CSV
                    </button>
                  </div>
                </div>

                <div>
                  <label class="block text-[11px] font-semibold text-[#64748B] mb-1.5">
                    入账至资产账户（{{ store.activeBook?.name || '当前账本' }}）
                  </label>
                  <CustomDropdown
                    v-model="targetWalletId"
                    :options="walletOptions"
                    placeholder="选择目标账户"
                  />
                </div>
              </div>

              <label
                class="block border-2 border-dashed border-[#D8DDE6] hover:border-[#DE8A1E] rounded-2xl p-6 text-center cursor-pointer bg-[#FAFBFD] hover:bg-[#FFFBF4] transition"
              >
                <div class="w-10 h-10 rounded-2xl bg-[#FFF6E8] text-[#DE8A1E] mx-auto flex items-center justify-center mb-2">
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="17 8 12 3 7 8" />
                    <line x1="12" y1="3" x2="12" y2="15" />
                  </svg>
                </div>
                <p class="font-bold text-[#18181B]">
                  {{ parsing ? '正在智能解析 CSV 账单…' : selectedFileName ? `已选择：${selectedFileName}` : `点击选择${platform === 'wechat' ? '微信支付' : '支付宝'}导出的 CSV 文件` }}
                </p>
                <p class="text-[11px] text-[#94A3B8] mt-1">
                  自动处理编码、过滤重复订单号并智能匹配支出/收入分类
                </p>
                <input
                  type="file"
                  accept=".csv,text/csv"
                  class="hidden"
                  :disabled="parsing"
                  @change="handleFileSelect"
                />
              </label>
            </div>

            <!-- 解析预览 -->
            <div v-if="parsedItems.length > 0" class="bg-white border border-[#EAECEF] rounded-2xl overflow-hidden">
              <div class="px-4 py-3 border-b border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2 bg-[#FAFBFD]">
                <div class="flex items-center gap-3">
                  <span class="font-bold text-[#18181B]">共解析 {{ importSummary.count }} 条有效流水</span>
                  <span class="text-[#D6553E] font-semibold tabular-nums">
                    支 {{ formatMoney(importSummary.expense) }}
                  </span>
                  <span class="text-[#3E8E63] font-semibold tabular-nums">
                    收 {{ formatMoney(importSummary.income) }}
                  </span>
                </div>
                <button
                  type="button"
                  class="text-[11px] text-[#64748B] hover:text-[#18181B]"
                  @click="resetImportState"
                >
                  清空重选
                </button>
              </div>

              <div class="max-h-60 overflow-y-auto thin-scroll">
                <table class="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
                      <th class="py-2 px-4 font-semibold">日期</th>
                      <th class="py-2 px-3 font-semibold">交易对方 / 商品说明</th>
                      <th class="py-2 px-4 font-semibold text-right">金额</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-[#F1F3F6]">
                    <tr v-for="(item, idx) in previewRows" :key="`${item.order_no}_${idx}`">
                      <td class="py-2.5 px-4 text-[#64748B] tabular-nums whitespace-nowrap">
                        {{ item.date }}
                      </td>
                      <td class="py-2.5 px-3 min-w-0">
                        <p class="font-semibold text-[#18181B] truncate max-w-[300px]">
                          {{ item.product || item.counterparty || '账单流水' }}
                        </p>
                        <p class="text-[10px] text-[#94A3B8] truncate max-w-[300px]">
                          {{ item.counterparty }}{{ item.note ? ` · ${item.note}` : '' }}
                        </p>
                      </td>
                      <td
                        class="py-2.5 px-4 text-right font-bold tabular-nums whitespace-nowrap"
                        :class="item.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                      >
                        {{ formatMoney(item.amount, { showSign: true, type: item.type }) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div
                v-if="parsedItems.length > 20"
                class="px-4 py-2 text-[11px] text-[#94A3B8] bg-[#FAFBFD] border-t border-[#F1F3F6]"
              >
                仅展示前 20 条预览，点击下方按钮将全部 {{ parsedItems.length }} 条导入账本
              </div>
            </div>
          </template>

          <!-- 导出面板 -->
          <template v-else>
            <div class="bg-white border border-[#EAECEF] rounded-2xl p-6 space-y-4">
              <div class="flex items-start gap-3.5">
                <div class="w-11 h-11 rounded-2xl bg-[#FFF6E8] text-[#DE8A1E] flex items-center justify-center shrink-0">
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
                  </svg>
                </div>
                <div class="space-y-1">
                  <h3 class="text-sm font-bold text-[#18181B]">
                    导出「{{ store.activeBook?.name || '当前账本' }}」完整流水 CSV
                  </h3>
                  <p class="text-[11px] text-[#64748B] leading-relaxed">
                    导出文件采用标准 UTF-8 编码（Excel 可直接打开不乱码），包含流水日期、收支类型、金额、一/二级分类、支付账户、商户标题与备注，方便本地归档与二次分析。
                  </p>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3 p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
                <div>
                  <p class="text-[10px] text-[#94A3B8]">目标账本</p>
                  <p class="text-xs font-bold text-[#18181B] mt-0.5 truncate">
                    {{ store.activeBook?.name || '默认账本' }}
                  </p>
                </div>
                <div>
                  <p class="text-[10px] text-[#94A3B8]">账本累计流水</p>
                  <p class="text-xs font-bold text-[#DE8A1E] tabular-nums mt-0.5">
                    {{ store.activeBook?.tx_count ?? store.transactionsTotal }} 笔
                  </p>
                </div>
              </div>

              <button
                type="button"
                :disabled="exporting"
                class="btn-amber w-full h-10 rounded-xl text-white text-xs font-semibold disabled:opacity-60 transition"
                @click="handleExportCsv"
              >
                {{ exporting ? '正在打包生成 CSV…' : '立即下载 CSV 账单文件' }}
              </button>
            </div>
          </template>
        </div>

        <!-- Footer -->
        <div class="px-5 py-3.5 bg-white border-t border-[#EAECEF] flex items-center justify-end gap-2 shrink-0">
          <button
            type="button"
            class="px-4 py-2 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#475569] text-xs font-semibold transition"
            @click="handleClose"
          >
            关闭
          </button>
          <button
            v-if="activeTab === 'import'"
            type="button"
            :disabled="committing || parsedItems.length === 0"
            class="btn-amber px-5 py-2 rounded-xl text-white text-xs font-semibold disabled:opacity-50 transition"
            @click="handleCommitImport"
          >
            {{ committing ? '正在入库…' : `确认导入 ${parsedItems.length} 条流水` }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
