<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney, formatActivityTime } from '@/utils/format';
import type { MonthlyReport, Transaction } from '@/types';
import type { PcViewId } from '@/pc/utils/adminAdapter';
import DonutChart from '@/components/shared/DonutChart.vue';
import type { DonutChartItem } from '@/components/shared/DonutChart.vue';
import TrendBarChart from '@/components/shared/TrendBarChart.vue';
import type { TrendBarItem } from '@/components/shared/TrendBarChart.vue';
import BudgetProgressBar from '@/components/shared/BudgetProgressBar.vue';
import AiChatPanel from '@/components/shared/AiChatPanel.vue';
import TransactionRow from '@/components/shared/TransactionRow.vue';

const emit = defineEmits<{
  navigate: [view: PcViewId];
  'edit-tx': [tx: Transaction];
  'open-record': [opts?: { mode?: 'ai' | 'manual'; defaultDate?: string; prompt?: string }];
  'open-import-export': [];
}>();

const store = useAppStore();

const monthlyReport = ref<MonthlyReport | null>(null);
const highlightedCatId = ref<string | null>(null);
const selectedTrendKey = ref<string | null>(null);

const currentMonthPrefix = computed(
  () => `${store.currentYear}-${String(store.currentMonth).padStart(2, '0')}-`
);

async function loadMonthlyReport() {
  if (!store.activeBookId) {
    monthlyReport.value = null;
    return;
  }
  try {
    monthlyReport.value = await api.getMonthlyReport(
      store.activeBookId,
      store.currentYear,
      store.currentMonth
    );
  } catch {
    monthlyReport.value = null;
  }
}

watch(
  () => [store.activeBookId, store.currentYear, store.currentMonth, store.transactionsTotal] as const,
  () => {
    loadMonthlyReport();
  },
  { immediate: true }
);

const monthExpense = computed<number>(() => {
  if (monthlyReport.value) return monthlyReport.value.total_expense || 0;
  if (store.dailyReport.length > 0) {
    return store.dailyReport.reduce((sum, d) => sum + (d.expense || 0), 0);
  }
  return store.transactions
    .filter((t) => t.type === 'expense' && t.date.startsWith(currentMonthPrefix.value))
    .reduce((sum, t) => sum + (t.amount || 0), 0);
});

const monthIncome = computed<number>(() => {
  if (monthlyReport.value) return monthlyReport.value.total_income || 0;
  if (store.dailyReport.length > 0) {
    return store.dailyReport.reduce((sum, d) => sum + (d.income || 0), 0);
  }
  return store.transactions
    .filter((t) => t.type === 'income' && t.date.startsWith(currentMonthPrefix.value))
    .reduce((sum, t) => sum + (t.amount || 0), 0);
});

const monthBalance = computed<number>(() => monthIncome.value - monthExpense.value);

const savingsRate = computed<number>(() => {
  if (monthIncome.value <= 0) return 0;
  return Math.max(0, Math.round((monthBalance.value / monthIncome.value) * 100));
});

const budgetLimit = computed<number>(
  () => store.budgets?.total_budget ?? store.activeBook?.budget ?? 0
);

const budgetSpent = computed<number>(
  () => store.budgets?.total_spent ?? monthExpense.value
);

const budgetPercent = computed<number>(() => {
  if (store.budgets) return Math.round(store.budgets.percentage || 0);
  if (budgetLimit.value <= 0) return 0;
  return Math.round((budgetSpent.value / budgetLimit.value) * 100);
});

// 预算提醒文案（store.latestBudgetAlert 由 SSE budget-alert 事件写入）。
const budgetAlertText = computed(() => {
  const alert = store.latestBudgetAlert;
  if (!alert) return '';
  return `${alert.budgetName}预算已使用 ${alert.level}%（${formatMoney(alert.used)} / ${formatMoney(alert.budget)}）`;
});

// 动态行左侧圆点颜色：交易琥珀、预算提醒红、成员绿、其他灰。
function activityColor(action: string): string {
  if (action === 'budget_alert') return '#D6553E';
  if (action.startsWith('transaction') || action === 'import_committed' || action === 'recurring_posted') return '#DE8A1E';
  if (action.startsWith('member')) return '#3E8E63';
  return '#94A3B8';
}

const trendItems = computed<TrendBarItem[]>(() => {
  const daysInMonth = new Date(store.currentYear, store.currentMonth, 0).getDate();
  const now = new Date();
  const isCurrentYM =
    now.getFullYear() === store.currentYear && now.getMonth() + 1 === store.currentMonth;
  const endDay = isCurrentYM ? Math.min(daysInMonth, Math.max(14, now.getDate())) : daysInMonth;
  const startDay = Math.max(1, endDay - 13);

  const dayExpenseMap = new Map<string, number>();
  for (const item of store.dailyReport) {
    if (item.date) {
      dayExpenseMap.set(item.date.slice(0, 10), item.expense || 0);
    }
  }

  const result: TrendBarItem[] = [];
  for (let d = startDay; d <= endDay; d++) {
    const iso = `${currentMonthPrefix.value}${String(d).padStart(2, '0')}`;
    result.push({
      key: iso,
      label: `${d}日`,
      value: dayExpenseMap.get(iso) || 0,
      rawDate: iso,
    });
  }
  return result;
});

const selectedTrendItem = computed<TrendBarItem | null>(() => {
  if (!selectedTrendKey.value) return null;
  return trendItems.value.find((it) => it.key === selectedTrendKey.value) || null;
});

const donutItems = computed<DonutChartItem[]>(() => {
  const cats = monthlyReport.value?.categories || [];
  return cats.map((c) => ({
    category_id: c.category_id,
    category_name: c.category_name,
    category_color: c.category_color || '#DE8A1E',
    amount: c.amount,
    percentage: c.percentage,
  }));
});

const recentTransactions = computed<Transaction[]>(() =>
  store.transactions.slice(0, 8)
);

function handleSelectTrendBar(item: TrendBarItem) {
  selectedTrendKey.value = selectedTrendKey.value === item.key ? null : item.key;
}
</script>

<template>
  <div class="space-y-5">
    <!-- 顶部 4 列 KPI 卡片 -->
    <section class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div class="pc-card pc-card-hover p-5 transition">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs text-[#64748B]">本月总支出</p>
          <span
            class="text-[10px] font-semibold px-2 py-0.5 rounded-full tabular-nums"
            :class="
              budgetPercent >= 85
                ? 'bg-[#FDF0EE] text-[#D6553E]'
                : 'bg-[#FFF6E8] text-[#DE8A1E]'
            "
          >
            预算 {{ budgetPercent }}%
          </span>
        </div>
        <p class="text-2xl font-bold text-[#D6553E] tabular-nums mt-1.5">
          {{ formatMoney(monthExpense, { privacy: store.privacyHidden }) }}
        </p>
        <p class="text-[11px] text-[#94A3B8] mt-2">
          {{ store.currentYear }}年{{ store.currentMonth }}月累计支出
        </p>
      </div>

      <div class="pc-card pc-card-hover p-5 transition">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs text-[#64748B]">本月总收入</p>
          <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#ECF6F0] text-[#3E8E63]">
            入账稳定
          </span>
        </div>
        <p class="text-2xl font-bold text-[#3E8E63] tabular-nums mt-1.5">
          {{ formatMoney(monthIncome, { privacy: store.privacyHidden }) }}
        </p>
        <p class="text-[11px] text-[#64748B] truncate mt-2">
          {{ store.activeBook?.name || '当前账本' }}
        </p>
      </div>

      <div class="pc-card pc-card-hover p-5 transition">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs text-[#64748B]">本月净结余</p>
          <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#F4F5F8] text-[#475569] tabular-nums">
            储蓄率 {{ savingsRate }}%
          </span>
        </div>
        <p
          class="text-2xl font-bold tabular-nums mt-1.5"
          :class="monthBalance < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
        >
          {{ formatMoney(monthBalance, { privacy: store.privacyHidden }) }}
        </p>
        <p class="text-[11px] text-[#64748B] mt-2">
          本月收入减支出实时结余
        </p>
      </div>

      <div class="pc-card pc-card-hover p-5 transition">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs text-[#64748B]">账本净资产</p>
          <button
            type="button"
            class="text-[10px] font-semibold text-[#DE8A1E] hover:underline"
            @click="emit('navigate', 'finance')"
          >
            管理账户 →
          </button>
        </div>
        <p class="text-2xl font-bold text-[#18181B] tabular-nums mt-1.5">
          {{ formatMoney(store.walletSummary.net_assets, { privacy: store.privacyHidden }) }}
        </p>
        <p class="text-[11px] text-[#64748B] tabular-nums mt-2 truncate">
          资产 {{ formatMoney(store.walletSummary.total_assets, { privacy: store.privacyHidden, compact: true }) }}
          · 负债 {{ formatMoney(store.walletSummary.total_liabilities, { privacy: store.privacyHidden, compact: true }) }}
        </p>
      </div>
    </section>

    <!-- 主体左右两栏 -->
    <section class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
      <!-- 左侧主分析区 -->
      <div class="lg:col-span-7 xl:col-span-8 space-y-5">
        <!-- 本月趋势柱状图 -->
        <div class="pc-card p-5">
          <div class="flex items-center justify-between gap-2 mb-2">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">
                {{ store.currentMonth }}月近期每日支出趋势
              </h2>
              <p class="text-[11px] text-[#94A3B8] mt-0.5">
                <template v-if="selectedTrendItem">
                  {{ selectedTrendItem.key }} 支出：
                  <strong class="text-[#18181B] tabular-nums">
                    {{ formatMoney(selectedTrendItem.value, { privacy: store.privacyHidden }) }}
                  </strong>
                </template>
                <template v-else>
                  点击任意柱条可锁定查看当日支出总额
                </template>
              </p>
            </div>
            <button
              type="button"
              class="text-xs font-semibold text-[#DE8A1E] hover:underline shrink-0"
              @click="emit('navigate', 'stats')"
            >
              完整报表 →
            </button>
          </div>

          <TrendBarChart
            mode="pc"
            :items="trendItems"
            :selected-key="selectedTrendKey"
            :privacy="store.privacyHidden"
            height-class="h-44"
            @select="handleSelectTrendBar"
          />
        </div>

        <!-- 分类支出占比 -->
        <div class="pc-card p-5">
          <div class="flex items-center justify-between gap-2 mb-4">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">本月分类支出构成</h2>
              <p class="text-[11px] text-[#94A3B8] mt-0.5">点击环形或图例可高亮对应分类</p>
            </div>
            <button
              type="button"
              class="text-xs font-semibold text-[#DE8A1E] hover:underline"
              @click="emit('navigate', 'categories')"
            >
              分类管理 →
            </button>
          </div>

          <div v-if="donutItems.length === 0" class="py-10 text-center text-xs text-[#94A3B8]">
            本月暂无支出记录，点击右上角「✦ 记一笔」开始记账
          </div>
          <DonutChart
            v-else
            :items="donutItems"
            :total="monthExpense"
            center-label="本月支出"
            :size="152"
            :highlighted-id="highlightedCatId"
            :privacy="store.privacyHidden"
            :max-legend-items="6"
            @select="highlightedCatId = $event"
          />
        </div>

        <!-- 最近流水明细 -->
        <div class="pc-card overflow-hidden">
          <div class="px-5 py-3.5 border-b border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">近期流水明细</h2>
              <p class="text-[11px] text-[#94A3B8]">点击任意记录可直接编辑或删除</p>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="text-xs font-semibold px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#18181B] transition"
                @click="emit('open-import-export')"
              >
                导入 / 导出 CSV
              </button>
              <button
                type="button"
                class="text-xs font-semibold text-[#DE8A1E] hover:underline"
                @click="emit('navigate', 'ledger')"
              >
                全部流水 →
              </button>
            </div>
          </div>

          <div v-if="recentTransactions.length === 0" class="py-12 text-center space-y-2.5">
            <p class="text-xs text-[#94A3B8]">当前账本尚无交易流水</p>
            <button
              type="button"
              class="px-4 py-1.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] text-xs font-semibold hover:bg-[#FBEBD1] transition"
              @click="emit('open-record', { mode: 'ai' })"
            >
              ✦ 用 AI 记下第一笔
            </button>
          </div>
          <div v-else class="divide-y divide-[#F1F3F6]">
            <TransactionRow
              v-for="tx in recentTransactions"
              :key="tx.id"
              :tx="tx"
              :privacy="store.privacyHidden"
              @click="emit('edit-tx', $event)"
            />
          </div>
        </div>
      </div>

      <!-- 右侧预算与 AI 快捷记账面板 -->
      <div class="lg:col-span-5 xl:col-span-4 space-y-5">
        <!-- 本月预算进度 -->
        <div class="pc-card p-5 space-y-3.5">
          <div class="flex items-center justify-between gap-2">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">月度预算水位</h2>
              <p class="text-[11px] text-[#64748B] mt-0.5 tabular-nums">
                总预算 {{ formatMoney(budgetLimit, { privacy: store.privacyHidden }) }}
              </p>
            </div>
            <button
              type="button"
              class="text-xs font-semibold text-[#DE8A1E] hover:underline"
              @click="emit('navigate', 'finance')"
            >
              调整预算 →
            </button>
          </div>

          <BudgetProgressBar
            :percentage="budgetPercent"
            :spent="budgetSpent"
            :limit="budgetLimit"
            :privacy="store.privacyHidden"
            show-summary
            height-class="h-2.5"
          />

          <!-- 预算提醒：SSE budget-alert 到达时出现，可关闭 -->
          <div
            v-if="store.latestBudgetAlert"
            class="flex items-center gap-2 rounded-xl bg-[#FDF1EE] border border-[#F6D9D2] px-3 py-2"
          >
            <span class="text-sm shrink-0">⚠️</span>
            <p class="flex-1 min-w-0 text-xs font-medium text-[#7A2E1D] truncate">
              {{ budgetAlertText }}
            </p>
            <button
              type="button"
              class="text-[#94A3B8] hover:text-[#18181B] text-xs shrink-0 px-1"
              @click="store.dismissBudgetAlert()"
            >
              ✕
            </button>
          </div>

          <div class="pt-3 border-t border-[#F1F3F6] grid grid-cols-2 gap-3 text-xs">
            <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <p class="text-[10px] text-[#94A3B8]">本月剩余天数</p>
              <p class="font-bold text-[#18181B] tabular-nums mt-0.5">
                {{ store.budgets?.remaining_days ?? 0 }} 天
              </p>
            </div>
            <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <p class="text-[10px] text-[#94A3B8]">建议日均可用</p>
              <p class="font-bold text-[#DE8A1E] tabular-nums mt-0.5">
                {{ formatMoney(store.budgets?.daily_allowance ?? 0, { privacy: store.privacyHidden }) }}
              </p>
            </div>
          </div>
        </div>

        <!-- 账本动态：谁干了什么，按时间倒序；SSE 到达时自动刷新 -->
        <div class="pc-card p-5 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <h2 class="text-sm font-bold text-[#18181B]">账本动态</h2>
            <span class="text-[11px] text-[#94A3B8]">{{ store.activities.length }} 条</span>
          </div>
          <div v-if="store.activities.length === 0" class="py-4 text-center text-xs text-[#94A3B8]">
            还没有动态，记一笔就出现在这里
          </div>
          <ul v-else class="divide-y divide-[#F1F3F6]">
            <li
              v-for="a in store.activities"
              :key="a.id"
              class="py-2.5 flex items-start gap-2.5 first:pt-0 last:pb-0"
            >
              <span
                class="mt-1.5 w-2 h-2 rounded-full shrink-0"
                :style="{ backgroundColor: activityColor(a.action) }"
              />
              <div class="flex-1 min-w-0">
                <p class="text-xs text-[#27272A] leading-5">
                  <span class="font-semibold">{{ a.actor_name }}</span>
                  {{ a.summary }}
                </p>
                <p class="text-[10px] text-[#94A3B8] mt-0.5">{{ formatActivityTime(a.created_at) }}</p>
              </div>
            </li>
          </ul>
          <button
            v-if="store.activitiesHasMore"
            type="button"
            class="w-full py-2 text-xs font-semibold text-[#DE8A1E] border-t border-[#F1F3F6] hover:bg-[#FFF6E8] rounded-b-xl transition disabled:opacity-60"
            :disabled="store.activitiesLoading"
            @click="store.loadMoreActivities()"
          >
            {{ store.activitiesLoading ? '加载中…' : '加载更多' }}
          </button>
        </div>

        <!-- 快捷 AI 记账迷你面板 -->
        <div class="pc-card overflow-hidden flex flex-col">
          <div class="px-5 py-3.5 border-b border-[#F1F3F6] flex items-center justify-between bg-white">
            <div class="flex items-center gap-2">
              <span class="text-[#DE8A1E] text-sm">✦</span>
              <div>
                <h2 class="text-sm font-bold text-[#18181B]">AI 快捷记账</h2>
                <p class="text-[10px] text-[#94A3B8]">对话、语音或小票图片一键入账</p>
              </div>
            </div>
            <button
              type="button"
              class="text-[11px] font-semibold text-[#64748B] hover:text-[#18181B]"
              @click="store.clearAiMessages()"
            >
              清空对话
            </button>
          </div>
          <AiChatPanel
            mode="pc"
            :compact="true"
            :book-id="store.activeBookId"
          />
        </div>
      </div>
    </section>
  </div>
</template>
