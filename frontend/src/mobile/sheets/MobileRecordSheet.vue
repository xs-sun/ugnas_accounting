<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useAppStore } from '@/stores/useAppStore';
import { evalSimpleExpr, formatMoney, todayIso, nowTimeHm, yuanToCents } from '@/utils/format';
import type { Transaction } from '@/types';
import type { RecordInitialMode } from '../types';
import BottomSheet from '../components/BottomSheet.vue';
import MobileKeypad from '../components/MobileKeypad.vue';
import AiChatPanel from '@/components/shared/AiChatPanel.vue';
import CategoryPicker from '@/components/shared/CategoryPicker.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    initialMode?: RecordInitialMode;
    prefillDate?: string;
  }>(),
  {
    initialMode: 'ai',
    prefillDate: '',
  }
);

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
  'view-tx': [txId: string];
}>();

const store = useAppStore();

// 滑动面板索引：0 = AI 聊着记，1 = 手动支出，2 = 手动收入
const slideIndex = ref<0 | 1 | 2>(0);
const isAiMode = computed(() => slideIndex.value === 0);
const manualType = computed<'expense' | 'income'>(() =>
  slideIndex.value === 2 ? 'income' : 'expense'
);

// 手动记账表单状态
const categoryId = ref('');
const subCategory = ref('');
const amountExpr = ref('');
const note = ref('');
const txDate = ref(todayIso());
const txTime = ref(nowTimeHm());
const selectedWalletId = ref('');
const includeBudget = ref(true);
const reimburse = ref(false);
const showMoreMeta = ref(false);
const submitting = ref(false);

// 水平滑动切换手势状态
const isSwiping = ref(false);
const swipeOffsetX = ref(0);
const transitionEnterX = ref(0);
let swipeStartX = 0;
let swipeStartY = 0;
let swipeDx = 0;
let swipeTracking = false;
let lockedAxis: 'x' | 'y' | null = null;

const categoryTree = computed(() =>
  manualType.value === 'income' ? store.incomeCategories : store.expenseCategories
);

const selectedWalletName = computed(() => {
  const found = store.activeWallets.find((w) => w.id === selectedWalletId.value);
  return found?.name || store.activeWallets[0]?.name || '选择账户';
});

// 生成最近 4 天的快捷日期列表（供点击胶囊快速轮换）
const quickDates = computed(() => {
  const base = props.prefillDate ? new Date(`${props.prefillDate}T12:00:00`) : new Date();
  const anchor = Number.isNaN(base.getTime()) ? new Date() : base;
  const list: Array<{ iso: string; label: string }> = [];
  for (let i = 0; i < 4; i += 1) {
    const d = new Date(anchor);
    d.setDate(anchor.getDate() - i);
    const iso = todayIso(d);
    const label = `${d.getMonth() + 1}月${d.getDate()}日`;
    list.push({ iso, label });
  }
  return list;
});

const datePillLabel = computed(() => {
  const iso = txDate.value || todayIso();
  const parts = iso.split('-');
  if (parts.length === 3) {
    return `${Number(parts[1])}月${Number(parts[2])}日`;
  }
  return iso;
});

function resetManualFormForOpen() {
  amountExpr.value = '';
  note.value = '';
  txDate.value = props.prefillDate || todayIso();
  txTime.value = nowTimeHm();
  includeBudget.value = true;
  reimburse.value = false;
  showMoreMeta.value = false;

  if (
    !selectedWalletId.value ||
    !store.activeWallets.some((w) => w.id === selectedWalletId.value)
  ) {
    selectedWalletId.value = store.activeWallets[0]?.id || '';
  }

  const tree = categoryTree.value;
  if (tree.length > 0 && !tree.some((c) => c.id === categoryId.value)) {
    categoryId.value = tree[0].id;
    subCategory.value = tree[0].children?.[0]?.name || '';
  }
}

watch(
  () => props.open,
  (opened) => {
    if (opened) {
      if (props.initialMode === 'expense') slideIndex.value = 1;
      else if (props.initialMode === 'income') slideIndex.value = 2;
      else slideIndex.value = 0;

      swipeOffsetX.value = 0;
      transitionEnterX.value = 0;
      isSwiping.value = false;
      resetManualFormForOpen();
    }
  },
  { immediate: true }
);

watch(manualType, (nextType) => {
  const tree = nextType === 'income' ? store.incomeCategories : store.expenseCategories;
  if (tree.length > 0 && !tree.some((c) => c.id === categoryId.value)) {
    categoryId.value = tree[0].id;
    subCategory.value = tree[0].children?.[0]?.name || '';
  }
});

function closeSheet() {
  emit('update:open', false);
  emit('close');
}

function goToSlide(idx: 0 | 1 | 2) {
  slideIndex.value = idx;
}

function switchManualButton() {
  if (slideIndex.value === 0) {
    slideIndex.value = 1;
  }
}

function cycleQuickDate() {
  const list = quickDates.value;
  if (!list.length) return;
  const idx = list.findIndex((item) => item.iso === txDate.value);
  const next = list[(idx + 1) % list.length];
  txDate.value = next.iso;
}

function cycleWallet() {
  const wallets = store.activeWallets;
  if (!wallets.length) {
    store.showToast('当前账本暂无可用账户，请先添加账户', 'error');
    return;
  }
  const idx = wallets.findIndex((w) => w.id === selectedWalletId.value);
  const next = wallets[(idx + 1) % wallets.length];
  selectedWalletId.value = next.id;
}

// 水平滑动触摸控制器（AI ⇄ 支出 ⇄ 收入）
function onStagePointerDown(e: PointerEvent) {
  if (e.button !== undefined && e.button !== 0) return;
  const target = e.target as HTMLElement | null;
  if (
    target?.closest(
      '.sheet-drag-handle, input, select, textarea, .key-btn, [data-no-swipe]'
    )
  ) {
    return;
  }
  swipeTracking = true;
  lockedAxis = null;
  swipeStartX = e.clientX;
  swipeStartY = e.clientY;
  swipeDx = 0;
}

function onStagePointerMove(e: PointerEvent) {
  if (!swipeTracking) return;
  const curDx = e.clientX - swipeStartX;
  const curDy = e.clientY - swipeStartY;

  if (!lockedAxis) {
    if (Math.abs(curDx) < 10 && Math.abs(curDy) < 10) return;
    lockedAxis = Math.abs(curDx) > Math.abs(curDy) * 1.2 ? 'x' : 'y';
    if (lockedAxis === 'y') {
      swipeTracking = false;
      return;
    }
    isSwiping.value = true;
  }

  swipeDx = curDx;
  const idx = slideIndex.value;
  const atLeftEdge = idx === 0 && swipeDx > 0;
  const atRightEdge = idx === 2 && swipeDx < 0;

  if (atLeftEdge || atRightEdge) {
    swipeOffsetX.value =
      Math.sign(swipeDx) * Math.min(48, Math.pow(Math.abs(swipeDx), 0.78) * 1.35);
  } else {
    swipeOffsetX.value = swipeDx * 0.55;
  }
}

function onStagePointerEnd() {
  if (!swipeTracking) return;
  swipeTracking = false;

  if (lockedAxis !== 'x') {
    isSwiping.value = false;
    swipeOffsetX.value = 0;
    return;
  }

  const idx = slideIndex.value;
  const threshold = 52;
  if (swipeDx < -threshold && idx < 2) {
    slideIndex.value = (idx + 1) as 0 | 1 | 2;
    isSwiping.value = true;
    swipeOffsetX.value = 36;
    requestAnimationFrame(() => {
      isSwiping.value = false;
      swipeOffsetX.value = 0;
    });
  } else if (swipeDx > threshold && idx > 0) {
    slideIndex.value = (idx - 1) as 0 | 1 | 2;
    isSwiping.value = true;
    swipeOffsetX.value = -36;
    requestAnimationFrame(() => {
      isSwiping.value = false;
      swipeOffsetX.value = 0;
    });
  } else {
    isSwiping.value = false;
    swipeOffsetX.value = 0;
  }

  lockedAxis = null;
  swipeDx = 0;
}

const stageStyle = computed(() => {
  const x = swipeOffsetX.value || transitionEnterX.value;
  return x ? { transform: `translateX(${x}px)` } : undefined;
});

async function handleManualSubmit(keepOpen: boolean) {
  if (submitting.value) return;

  const yuan = evalSimpleExpr(amountExpr.value);
  if (!yuan || yuan <= 0) {
    store.showToast('请先输入大于 0 的金额', 'error');
    return;
  }

  if (!store.activeBookId) {
    store.showToast('请先选择或创建一个账本', 'error');
    return;
  }

  const walletId = selectedWalletId.value || store.activeWallets[0]?.id || '';
  if (!walletId) {
    store.showToast('当前账本暂无可用账户，请先在「我的 -> 资产账户」中添加', 'error');
    return;
  }

  const cents = yuanToCents(yuan);
  const activeCat = categoryTree.value.find((c) => c.id === categoryId.value);
  const trimmedNote = note.value.trim();
  const title = trimmedNote || subCategory.value || activeCat?.name || '记一笔';

  submitting.value = true;
  try {
    await store.createTransaction({
      book_id: store.activeBookId,
      type: manualType.value,
      amount: cents,
      date: txDate.value || todayIso(),
      tx_time: txTime.value || nowTimeHm(),
      wallet_id: walletId,
      category_id: categoryId.value || undefined,
      title,
      sub_category: subCategory.value || '',
      note: trimmedNote,
      include_budget: manualType.value === 'expense' ? includeBudget.value : false,
      reimburse: manualType.value === 'expense' ? reimburse.value : false,
    });

    const formatted = formatMoney(cents, {
      showSign: true,
      type: manualType.value,
    });
    store.showToast(`已记录 ${activeCat?.name || ''} ${formatted}`, 'success');

    amountExpr.value = '';
    note.value = '';
    txTime.value = nowTimeHm();
    if (!keepOpen) {
      closeSheet();
    }
  } catch (err: any) {
    store.showToast(err?.message || '保存记账失败，请重试', 'error');
  } finally {
    submitting.value = false;
  }
}

function onAiRecorded(_tx: Transaction) {
  // AiChatPanel 内部已自动刷新 store 并展示“已入账”状态卡片
}

function onAiViewTx(txId: string) {
  closeSheet();
  emit('view-tx', txId);
}
</script>

<template>
  <BottomSheet
    :open="open"
    :multi-detent="true"
    max-height-class="max-h-[92dvh]"
    bg-class="bg-[#F8F9FB]"
    z-index-class="z-40"
    @close="emit('close')"
    @update:open="emit('update:open', $event)"
  >
    <!-- 顶部模式切换药丸 + 3 点滑动指示器 + 关闭按钮 -->
    <template #header>
      <div class="px-4 py-2 flex items-center justify-between gap-2 shrink-0">
        <div class="inline-flex items-center p-1 rounded-full bg-[#ECEEF2]">
          <button
            type="button"
            class="px-3.5 py-1.5 rounded-full text-xs font-semibold transition flex items-center gap-1.5"
            :class="
              isAiMode
                ? 'bg-white text-[#DE8A1E] shadow-xs'
                : 'text-[#64748B] hover:text-[#18181B]'
            "
            @click="goToSlide(0)"
          >
            <span>✦</span>
            <span>AI 聊着记</span>
          </button>
          <button
            type="button"
            class="px-3.5 py-1.5 rounded-full text-xs font-semibold transition"
            :class="
              !isAiMode
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B] hover:text-[#18181B]'
            "
            @click="switchManualButton"
          >
            手动记一笔
          </button>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <div class="flex items-center gap-1" title="左右滑动切换：AI ⇄ 支出 ⇄ 收入">
            <button
              v-for="dotIdx in ([0, 1, 2] as const)"
              :key="dotIdx"
              type="button"
              :aria-label="dotIdx === 0 ? 'AI 记账' : dotIdx === 1 ? '手动支出' : '手动收入'"
              class="h-1.5 rounded-full transition-all"
              :class="
                slideIndex === dotIdx
                  ? 'w-3.5 bg-[#DE8A1E]'
                  : 'w-1.5 bg-[#D4D9E1]'
              "
              @click="goToSlide(dotIdx)"
            />
          </div>

          <button
            type="button"
            aria-label="关闭记账抽屉"
            class="w-8 h-8 rounded-full bg-white border border-[#E6E9EF] flex items-center justify-center text-[#64748B] hover:text-[#18181B] transition"
            @click="closeSheet"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </template>

    <!-- 可水平滑动舞台：AI 聊着记 ⇄ 手动支出 ⇄ 手动收入 -->
    <div
      class="record-swipe-stage flex-1 min-h-0 flex flex-col overflow-hidden"
      :class="{ 'is-swiping': isSwiping }"
      :style="stageStyle"
      @pointerdown="onStagePointerDown"
      @pointermove="onStagePointerMove"
      @pointerup="onStagePointerEnd"
      @pointercancel="onStagePointerEnd"
    >
      <!-- 面板 A：AI 聊着记 -->
      <div v-if="isAiMode" class="flex-1 min-h-0 flex flex-col overflow-hidden">
        <AiChatPanel
          mode="mobile"
          :book-id="store.activeBookId"
          @recorded="onAiRecorded"
          @view-tx="onAiViewTx"
        />
      </div>

      <!-- 面板 B / C：手动支出 / 手动收入 -->
      <div v-else class="flex-1 min-h-0 flex flex-col overflow-hidden">
        <!-- 支出 / 收入切换条 + 当前账本指示 -->
        <div class="px-4 pb-2 flex items-center justify-between shrink-0">
          <div class="inline-flex p-0.5 rounded-full bg-[#ECEEF2] text-xs font-semibold">
            <button
              type="button"
              class="px-4 py-1 rounded-full transition"
              :class="
                manualType === 'expense'
                  ? 'bg-white text-[#D6553E] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="goToSlide(1)"
            >
              支出
            </button>
            <button
              type="button"
              class="px-4 py-1 rounded-full transition"
              :class="
                manualType === 'income'
                  ? 'bg-white text-[#3E8E63] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="goToSlide(2)"
            >
              收入
            </button>
          </div>
          <span class="text-[11px] font-medium text-[#64748B] truncate max-w-[160px]">
            {{ store.activeBook?.name || '默认账本' }}
          </span>
        </div>

        <!-- 分类选择 + 可展开更多日期时间设置 -->
        <div class="flex-1 min-h-0 overflow-y-auto phone-scroll px-4 pb-2 space-y-2.5">
          <div class="bg-white border border-[#ECEEF2] rounded-2xl p-3" data-no-swipe>
            <CategoryPicker
              v-model="categoryId"
              v-model:sub-category="subCategory"
              :type="manualType"
              :tree="categoryTree"
              :columns="5"
            />
          </div>

          <div
            v-if="showMoreMeta"
            class="bg-white border border-[#ECEEF2] rounded-2xl divide-y divide-[#F1F3F6] text-xs"
            data-no-swipe
          >
            <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
              <span class="text-[#64748B]">记账日期</span>
              <input
                v-model="txDate"
                type="date"
                class="text-right text-xs font-semibold text-[#18181B] bg-transparent outline-none"
              />
            </div>
            <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
              <span class="text-[#64748B]">记账时间</span>
              <input
                v-model="txTime"
                type="time"
                class="text-right text-xs font-semibold text-[#18181B] bg-transparent outline-none"
              />
            </div>
            <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
              <span class="text-[#64748B]">支付账户</span>
              <select
                v-model="selectedWalletId"
                class="text-right text-xs font-semibold text-[#18181B] bg-transparent outline-none"
              >
                <option v-for="w in store.activeWallets" :key="w.id" :value="w.id">
                  {{ w.name }}
                </option>
              </select>
            </div>
            <template v-if="manualType === 'expense'">
              <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
                <span class="text-[#64748B]">计入本月预算</span>
                <ToggleSwitch v-model="includeBudget" aria-label="计入本月预算" />
              </div>
              <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
                <span class="text-[#64748B]">标记为待报销</span>
                <ToggleSwitch v-model="reimburse" aria-label="标记为待报销" />
              </div>
            </template>
          </div>
        </div>

        <!-- 计算器数字键盘 -->
        <MobileKeypad
          v-model="amountExpr"
          :type="manualType"
          :submitting="submitting"
          @submit="handleManualSubmit"
        >
          <template #header-right>
            <input
              v-model="note"
              type="text"
              maxlength="40"
              placeholder="添加备注…"
              class="w-full text-right text-xs bg-transparent outline-none text-[#18181B] placeholder-[#94A3B8]"
            />
          </template>

          <template #meta>
            <div class="py-2 flex items-center gap-1.5 overflow-x-auto phone-scroll" data-no-swipe>
              <button
                type="button"
                class="shrink-0 text-[11px] font-medium px-2.5 py-1 rounded-full bg-[#F5F6F8] text-[#475569] hover:bg-[#ECEEF2] transition"
                @click="cycleQuickDate"
              >
                {{ datePillLabel }} · {{ txTime }}
              </button>

              <button
                type="button"
                class="shrink-0 text-[11px] font-medium px-2.5 py-1 rounded-full bg-[#F5F6F8] text-[#475569] hover:bg-[#ECEEF2] transition"
                @click="cycleWallet"
              >
                {{ selectedWalletName }}
              </button>

              <button
                v-if="manualType === 'expense'"
                type="button"
                class="shrink-0 text-[11px] px-2.5 py-1 rounded-full transition"
                :class="
                  includeBudget
                    ? 'font-semibold bg-[#ECF6F0] text-[#3E8E63]'
                    : 'font-medium bg-[#F5F6F8] text-[#94A3B8]'
                "
                @click="includeBudget = !includeBudget"
              >
                {{ includeBudget ? '计入预算' : '不计预算' }}
              </button>

              <button
                v-if="manualType === 'expense'"
                type="button"
                class="shrink-0 text-[11px] px-2.5 py-1 rounded-full transition"
                :class="
                  reimburse
                    ? 'font-semibold bg-[#FFF6E8] text-[#DE8A1E]'
                    : 'font-medium bg-[#F5F6F8] text-[#94A3B8]'
                "
                @click="reimburse = !reimburse"
              >
                待报销
              </button>

              <button
                type="button"
                class="shrink-0 text-[11px] font-medium px-2.5 py-1 rounded-full transition"
                :class="
                  showMoreMeta
                    ? 'bg-[#18181B] text-white'
                    : 'bg-[#F5F6F8] text-[#64748B] hover:bg-[#ECEEF2]'
                "
                @click="showMoreMeta = !showMoreMeta"
              >
                {{ showMoreMeta ? '收起时间' : '选日期/时间' }}
              </button>
            </div>
          </template>
        </MobileKeypad>
      </div>
    </div>
  </BottomSheet>
</template>
