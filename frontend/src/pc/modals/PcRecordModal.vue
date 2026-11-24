<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { centsToYuan, yuanToCents, todayIso, nowTimeHm } from '@/utils/format';
import type { Transaction, CategoryTreeNode } from '@/types';
import AiChatPanel from '@/components/shared/AiChatPanel.vue';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import CategoryPicker from '@/components/shared/CategoryPicker.vue';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    initialMode?: 'ai' | 'manual';
    editingTx?: Transaction | null;
    defaultDate?: string;
    initialAiPrompt?: string;
  }>(),
  {
    initialMode: 'ai',
    editingTx: null,
    defaultDate: '',
    initialAiPrompt: '',
  }
);

const emit = defineEmits<{
  close: [];
  saved: [tx: Transaction];
  deleted: [txId: string];
}>();

const store = useAppStore();

const mode = ref<'ai' | 'manual'>('ai');
const submitting = ref(false);
const deleting = ref(false);
const confirmDelete = ref(false);
const sendingInitialPrompt = ref(false);
const errorMsg = ref('');

const form = reactive({
  type: 'expense' as 'expense' | 'income',
  amountYuan: '',
  title: '',
  categoryId: '',
  subCategory: '',
  walletId: '',
  date: todayIso(),
  txTime: nowTimeHm(),
  note: '',
  includeBudget: true,
  reimburse: false,
});

const isEdit = computed(() => Boolean(props.editingTx));

const categoryTree = computed<CategoryTreeNode[]>(() =>
  form.type === 'income' ? store.incomeCategories : store.expenseCategories
);

const activeCategory = computed<CategoryTreeNode | undefined>(() =>
  categoryTree.value.find((c) => c.id === form.categoryId) || categoryTree.value[0]
);

const walletOptions = computed<DropdownOption[]>(() =>
  store.activeWallets.map((w) => ({
    value: w.id,
    label: w.name,
    dot: w.color || '#DE8A1E',
  }))
);

function resetManualForm() {
  errorMsg.value = '';
  confirmDelete.value = false;
  if (props.editingTx) {
    const tx = props.editingTx;
    form.type = tx.type === 'income' ? 'income' : 'expense';
    form.amountYuan = centsToYuan(tx.amount).toFixed(2);
    form.title = tx.title || '';
    form.categoryId = tx.category_id || '';
    form.subCategory = tx.sub_category || '';
    form.walletId = tx.wallet_id || store.activeWallets[0]?.id || '';
    form.date = tx.date || todayIso();
    form.txTime = tx.tx_time || nowTimeHm();
    form.note = tx.note || '';
    form.includeBudget = tx.include_budget ?? true;
    form.reimburse = tx.reimburse ?? false;
    return;
  }

  form.type = 'expense';
  form.amountYuan = '';
  form.title = '';
  const firstCat = store.expenseCategories[0];
  form.categoryId = firstCat?.id || '';
  form.subCategory = firstCat?.children?.[0]?.name || '';
  form.walletId = store.activeWallets[0]?.id || '';
  form.date = props.defaultDate || todayIso();
  form.txTime = nowTimeHm();
  form.note = '';
  form.includeBudget = true;
  form.reimburse = false;
}

async function sendInitialPromptIfNeeded(promptText: string) {
  const clean = promptText.trim();
  if (!clean || !store.activeBookId || sendingInitialPrompt.value) return;

  sendingInitialPrompt.value = true;
  store.appendAiMessage({
    id: `u_init_${Date.now()}`,
    role: 'user',
    text: clean,
    isVoice: false,
  });

  try {
    const res = await api.aiChat({
      book_id: store.activeBookId,
      message: clean,
      is_voice: false,
    });
    if (res.usage && store.user) {
      store.user = {
        ...store.user,
        today_ai_count: res.usage.today_count,
        daily_ai_limit: res.usage.daily_limit,
      };
    }
    store.appendAiMessage({
      id: `a_init_${Date.now() + 1}`,
      role: 'assistant',
      text: res.reply,
      draft: res.draft,
      saved: false,
      editing: false,
    });
  } catch (err: any) {
    const msg = err?.message || 'AI 解析暂不可用，请稍后重试';
    store.appendAiMessage({
      id: `a_init_err_${Date.now() + 1}`,
      role: 'assistant',
      text: `抱歉，${msg}`,
      draft: null,
    });
    store.showToast(msg, 'error');
  } finally {
    sendingInitialPrompt.value = false;
  }
}

watch(
  () => props.open,
  (val) => {
    if (!val) return;
    resetManualForm();
    if (props.editingTx) {
      mode.value = 'manual';
    } else {
      mode.value = props.initialMode || 'ai';
      if (mode.value === 'ai' && props.initialAiPrompt) {
        sendInitialPromptIfNeeded(props.initialAiPrompt);
      }
    }
  }
);

function switchType(nextType: 'expense' | 'income') {
  if (form.type === nextType) return;
  form.type = nextType;
  const list = nextType === 'income' ? store.incomeCategories : store.expenseCategories;
  const first = list[0];
  form.categoryId = first?.id || '';
  form.subCategory = first?.children?.[0]?.name || '';
}

function handleClose() {
  if (submitting.value || deleting.value) return;
  emit('close');
}

function handleAiRecorded(tx: Transaction) {
  emit('saved', tx);
}

async function handleSubmitManual() {
  if (submitting.value) return;
  errorMsg.value = '';

  if (!store.activeBookId) {
    errorMsg.value = '请先选择或创建账本';
    return;
  }

  const cents = yuanToCents(form.amountYuan);
  if (!cents || cents <= 0) {
    errorMsg.value = '请输入大于 0 的金额';
    return;
  }

  const walletId = form.walletId || store.activeWallets[0]?.id || '';
  if (!walletId) {
    errorMsg.value = '当前账本暂无可用资产账户，请先前往「资产与预算」创建账户';
    return;
  }

  const fallbackTitle =
    form.subCategory || activeCategory.value?.name || (form.type === 'income' ? '收入' : '支出');

  submitting.value = true;
  try {
    let savedTx: Transaction;
    if (props.editingTx) {
      savedTx = await store.updateTransaction(props.editingTx.id, {
        book_id: props.editingTx.book_id || store.activeBookId,
        type: form.type,
        amount: cents,
        date: form.date || todayIso(),
        tx_time: form.txTime || nowTimeHm(),
        wallet_id: walletId,
        category_id: form.categoryId || null,
        title: form.title.trim() || fallbackTitle,
        sub_category: form.subCategory,
        note: form.note.trim(),
        include_budget: form.includeBudget,
        reimburse: form.reimburse,
      });
      store.showToast(`已保存「${savedTx.title || fallbackTitle}」`, 'success');
    } else {
      savedTx = await store.createTransaction({
        book_id: store.activeBookId,
        type: form.type,
        amount: cents,
        date: form.date || todayIso(),
        tx_time: form.txTime || nowTimeHm(),
        wallet_id: walletId,
        category_id: form.categoryId || null,
        title: form.title.trim() || fallbackTitle,
        sub_category: form.subCategory,
        note: form.note.trim(),
        include_budget: form.includeBudget,
        reimburse: form.reimburse,
      });
      store.showToast(
        `已记录「${savedTx.title || fallbackTitle}」¥${centsToYuan(savedTx.amount).toFixed(2)}`,
        'success'
      );
    }

    emit('saved', savedTx);
    emit('close');
  } catch (err: any) {
    errorMsg.value = err?.message || '保存失败，请检查填写内容';
    store.showToast(errorMsg.value, 'error');
  } finally {
    submitting.value = false;
  }
}

async function handleDelete() {
  if (!props.editingTx || deleting.value) return;
  if (!confirmDelete.value) {
    confirmDelete.value = true;
    return;
  }

  deleting.value = true;
  try {
    const id = props.editingTx.id;
    await store.deleteTransaction(id);
    store.showToast('已删除该笔流水明细', 'success');
    emit('deleted', id);
    emit('close');
  } catch (err: any) {
    store.showToast(err?.message || '删除失败，请重试', 'error');
  } finally {
    deleting.value = false;
  }
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
        class="relative w-full max-w-[600px] max-h-[88vh] bg-white rounded-[26px] border border-[#EAECEF] shadow-2xl flex flex-col overflow-hidden"
      >
        <!-- 编辑模式 Header -->
        <div
          v-if="isEdit"
          class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between shrink-0 bg-white"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <CategoryBadge
              :icon="activeCategory?.icon || editingTx?.category_icon || 'tag'"
              :color="activeCategory?.color || editingTx?.category_color || '#DE8A1E'"
              :bg-color="activeCategory?.bg_color || editingTx?.category_bg_color || '#FFF6E8'"
              size="sm"
            />
            <div class="min-w-0">
              <h3 class="text-sm font-bold text-[#18181B]">编辑流水明细</h3>
              <p class="text-[11px] text-[#94A3B8] truncate">
                {{ store.activeBook?.name || '当前账本' }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <div class="p-0.5 rounded-full bg-[#F1F3F6] inline-flex text-[11px] font-semibold">
              <button
                type="button"
                class="px-3 py-1 rounded-full transition"
                :class="
                  form.type === 'expense'
                    ? 'bg-white text-[#D6553E] shadow-xs'
                    : 'text-[#64748B]'
                "
                @click="switchType('expense')"
              >
                支出
              </button>
              <button
                type="button"
                class="px-3 py-1 rounded-full transition"
                :class="
                  form.type === 'income'
                    ? 'bg-white text-[#3E8E63] shadow-xs'
                    : 'text-[#64748B]'
                "
                @click="switchType('income')"
              >
                收入
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
        </div>

        <!-- 新建双模式 Header -->
        <div
          v-else
          class="px-5 py-3.5 border-b border-[#F1F3F6] flex items-center justify-between gap-2 shrink-0 bg-white"
        >
          <div class="inline-flex items-center p-1 rounded-full bg-[#F1F3F6]">
            <button
              type="button"
              class="px-4 py-1.5 rounded-full text-xs font-semibold transition flex items-center gap-1.5"
              :class="
                mode === 'ai'
                  ? 'bg-white text-[#DE8A1E] shadow-xs'
                  : 'text-[#64748B] hover:text-[#18181B]'
              "
              @click="mode = 'ai'"
            >
              <span>✦</span>
              <span>AI 聊着记</span>
            </button>
            <button
              type="button"
              class="px-4 py-1.5 rounded-full text-xs font-semibold transition"
              :class="
                mode === 'manual'
                  ? 'bg-white text-[#18181B] shadow-xs'
                  : 'text-[#64748B] hover:text-[#18181B]'
              "
              @click="mode = 'manual'"
            >
              手动记一笔
            </button>
          </div>

          <div class="flex items-center gap-2.5 shrink-0">
            <span
              v-if="mode === 'ai'"
              class="hidden sm:inline-block text-[11px] text-[#64748B] bg-[#F4F5F8] px-2.5 py-1 rounded-full tabular-nums"
            >
              今日 AI {{ store.user?.today_ai_count ?? 0 }}/{{ store.user?.daily_ai_limit ?? 30 }}
            </span>
            <span v-else class="text-[11px] font-medium text-[#64748B] truncate max-w-[140px]">
              {{ store.activeBook?.name || '当前账本' }}
            </span>
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
        </div>

        <!-- Tab A: AI 聊着记 -->
        <div v-if="!isEdit && mode === 'ai'" class="flex-1 flex flex-col min-h-0 overflow-hidden">
          <div
            v-if="sendingInitialPrompt"
            class="px-5 py-2 bg-[#FFF6E8] border-b border-[#F4DFBD] text-[11px] font-medium text-[#B8680E] flex items-center gap-2"
          >
            <span class="w-2 h-2 rounded-full bg-[#DE8A1E] animate-pulse" />
            <span>正在为你向七猫 AI 发起分析…</span>
          </div>
          <AiChatPanel
            mode="pc"
            :book-id="store.activeBookId"
            @recorded="handleAiRecorded"
          />
        </div>

        <!-- Tab B: 手动记账 / 编辑 -->
        <template v-else>
          <form
            id="pcRecordManualForm"
            class="p-5 overflow-y-auto thin-scroll space-y-4 text-xs bg-[#F8F9FB] flex-1"
            @submit.prevent="handleSubmitManual"
          >
            <div
              v-if="errorMsg"
              class="px-3.5 py-2 rounded-xl bg-[#FDF0EE] border border-[#F5C4BC] text-[#D6553E] text-xs font-medium"
            >
              {{ errorMsg }}
            </div>

            <div v-if="!isEdit" class="flex items-center justify-between">
              <div class="p-0.5 rounded-full bg-[#ECEEF2] inline-flex text-xs font-semibold">
                <button
                  type="button"
                  class="px-4 py-1 rounded-full transition"
                  :class="
                    form.type === 'expense'
                      ? 'bg-white text-[#D6553E] shadow-xs'
                      : 'text-[#64748B]'
                  "
                  @click="switchType('expense')"
                >
                  支出
                </button>
                <button
                  type="button"
                  class="px-4 py-1 rounded-full transition"
                  :class="
                    form.type === 'income'
                      ? 'bg-white text-[#3E8E63] shadow-xs'
                      : 'text-[#64748B]'
                  "
                  @click="switchType('income')"
                >
                  收入
                </button>
              </div>
              <span class="text-[11px] text-[#64748B]">
                {{ activeCategory?.name || '未选分类'
                }}{{ form.subCategory ? ` · ${form.subCategory}` : '' }}
              </span>
            </div>

            <!-- 金额与名称 -->
            <div class="bg-white border border-[#EAECEF] rounded-2xl p-4 grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">金额（元）</label>
                <input
                  v-model="form.amountYuan"
                  type="number"
                  step="0.01"
                  min="0.01"
                  required
                  placeholder="0.00"
                  class="w-full h-10 text-lg font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
                  :class="form.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">名称 / 商户</label>
                <input
                  v-model="form.title"
                  type="text"
                  maxlength="40"
                  placeholder="如：瑞幸拿铁、盒马鲜生"
                  class="w-full h-10 font-semibold bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
                />
              </div>
            </div>

            <!-- 分类选择 -->
            <div class="bg-white border border-[#EAECEF] rounded-2xl p-3.5">
              <CategoryPicker
                v-model="form.categoryId"
                v-model:sub-category="form.subCategory"
                :type="form.type"
                :tree="categoryTree"
                :columns="5"
              />
            </div>

            <!-- 日期、时间与支付账户 -->
            <div class="bg-white border border-[#EAECEF] rounded-2xl p-4 space-y-3">
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label class="block text-[11px] font-semibold text-[#64748B] mb-1">日期</label>
                  <input
                    v-model="form.date"
                    type="date"
                    required
                    class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 font-semibold outline-none focus:border-[#DE8A1E] focus:bg-white"
                  />
                </div>
                <div>
                  <label class="block text-[11px] font-semibold text-[#64748B] mb-1">时间</label>
                  <input
                    v-model="form.txTime"
                    type="time"
                    class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 font-semibold outline-none focus:border-[#DE8A1E] focus:bg-white"
                  />
                </div>
                <div>
                  <label class="block text-[11px] font-semibold text-[#64748B] mb-1">支付账户</label>
                  <CustomDropdown
                    v-model="form.walletId"
                    :options="walletOptions"
                    placeholder="选择账户"
                  />
                </div>
              </div>

              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">备注说明</label>
                <input
                  v-model="form.note"
                  type="text"
                  maxlength="100"
                  placeholder="可选备注信息，支持后续关键词搜索"
                  class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>

              <div class="pt-2 border-t border-[#F1F3F6] grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="flex items-center justify-between px-3 py-2 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
                  <div>
                    <p class="font-semibold text-[#18181B]">计入本月预算</p>
                    <p class="text-[10px] text-[#94A3B8]">关闭后不影响总预算消耗统计</p>
                  </div>
                  <ToggleSwitch v-model="form.includeBudget" aria-label="计入本月预算" />
                </div>

                <div class="flex items-center justify-between px-3 py-2 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
                  <div>
                    <p class="font-semibold text-[#18181B]">标记可报销</p>
                    <p class="text-[10px] text-[#94A3B8]">便于在流水中筛选待报销项</p>
                  </div>
                  <ToggleSwitch v-model="form.reimburse" aria-label="标记可报销" />
                </div>
              </div>
            </div>
          </form>

          <!-- Footer -->
          <div class="px-5 py-3.5 bg-white border-t border-[#EAECEF] flex items-center justify-between gap-2 shrink-0">
            <div v-if="isEdit" class="flex items-center gap-2">
              <button
                type="button"
                :disabled="deleting"
                class="px-4 py-2 rounded-xl text-xs font-semibold transition"
                :class="
                  confirmDelete
                    ? 'bg-[#D6553E] text-white hover:bg-[#BE4530]'
                    : 'bg-[#FDF0EE] text-[#D6553E] hover:bg-[#FADCD7]'
                "
                @click="handleDelete"
              >
                {{ deleting ? '删除中…' : confirmDelete ? '再次点击确认删除' : '删除流水' }}
              </button>
              <button
                v-if="confirmDelete"
                type="button"
                class="px-2.5 py-2 rounded-xl text-xs text-[#64748B] hover:bg-[#F4F5F8]"
                @click="confirmDelete = false"
              >
                取消删除
              </button>
            </div>
            <span v-else />

            <div class="flex items-center gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#475569] text-xs font-semibold transition"
                @click="handleClose"
              >
                取消
              </button>
              <button
                type="submit"
                form="pcRecordManualForm"
                :disabled="submitting"
                class="btn-amber px-5 py-2 rounded-xl text-white text-xs font-semibold disabled:opacity-60 transition"
              >
                {{ submitting ? '保存中…' : isEdit ? '保存修改' : '确认记账' }}
              </button>
            </div>
          </div>
        </template>
      </div>
    </div>
  </Teleport>
</template>
