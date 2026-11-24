<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { DEFAULT_MASCOT, MASCOT_FALLBACK } from '@/constants/avatars';
import { formatMoney } from '@/utils/format';
import type { AiInsightsResponse, CategoryReportItem, StatsPeriod, Transaction } from '@/types';
import DonutChart, { type DonutChartItem } from '@/components/shared/DonutChart.vue';
import TrendBarChart, { type TrendBarItem } from '@/components/shared/TrendBarChart.vue';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';

const emit = defineEmits<{
  'select-tx': [tx: Transaction];
  'open-ai': [];
}>();

const store = useAppStore();

const highlightedCatId = ref<string | null>(null);
const selectedBarKey = ref<string | null>(null);
const loadingStats = ref(false);

// AI 消费洞察状态
const insightSeed = ref(0);
const loadingInsight = ref(false);
const insights = ref<AiInsightsResponse | null>(null);

const PERIOD_OPTIONS: Array<{ id: StatsPeriod; label: string }> = [
  { id: 'week', label: '本周' },
  { id: 'month', label: '本月' },
  { id: 'year', label: '全年' },
];

const statsData = computed(() => store.stats);

const periodLabel = computed(() => {
  if (statsData.value?.start_date && statsData.value?.end_date) {
    return `${statsData.value.start_date} ~ ${statsData.value.end_date}`;
  }
  if (store.statsPeriod === 'year') {
    return `${store.currentYear}年 全年累计`;
  }
  if (store.statsPeriod === 'week') {
    return '近 7 天收支统计';
  }
  return `${store.currentYear}年${store.currentMonth}月`;
});

const totalExpense = computed(() => statsData.value?.total_expense || 0);
const totalIncome = computed(() => statsData.value?.total_income || 0);
const netSavings = computed(() => statsData.value?.net_savings ?? totalIncome.value - totalExpense.value);

const activeTotal = computed(() =>
  store.statsType === 'income' ? totalIncome.value : totalExpense.value
);

// 计算当前周期天数用于日均均值
const periodDays = computed(() => {
  const s = statsData.value;
  if (s?.start_date && s?.end_date) {
    const start = new Date(`${s.start_date}T00:00:00`);
    const end = new Date(`${s.end_date}T00:00:00`);
    const diff = Math.round((end.getTime() - start.getTime()) / 86400000) + 1;
    if (Number.isFinite(diff) && diff > 0) return diff;
  }
  if (s?.daily_series?.length) return s.daily_series.length;
  return store.statsPeriod === 'week' ? 7 : store.statsPeriod === 'year' ? 365 : 30;
});

const dailyAverage = computed(() => {
  if (activeTotal.value <= 0) return 0;
  return Math.round(activeTotal.value / Math.max(1, periodDays.value));
});

const categoryBreakdown = computed<CategoryReportItem[]>(() => {
  const list = statsData.value?.categories || [];
  return [...list].sort((a, b) => (b.amount || 0) - (a.amount || 0));
});

const totalTxCount = computed(() =>
  categoryBreakdown.value.reduce((sum, item) => sum + Number(item.count || 0), 0)
);

const donutItems = computed<DonutChartItem[]>(() =>
  categoryBreakdown.value.map((item) => ({
    category_id: item.category_id,
    category_name: item.category_name,
    category_color: item.category_color || '#DE8A1E',
    amount: item.amount || 0,
    percentage: item.percentage || 0,
  }))
);

const trendItems = computed<TrendBarItem[]>(() => {
  const series = statsData.value?.daily_series || [];
  return series.map((item) => ({
    key: item.date || item.label,
    label: item.label || item.date?.slice(5) || '',
    value: store.statsType === 'income' ? item.income : item.expense,
    rawDate: item.date,
  }));
});

const activeBarItem = computed<TrendBarItem | null>(() => {
  if (!trendItems.value.length) return null;
  if (selectedBarKey.value) {
    const found = trendItems.value.find((it) => it.key === selectedBarKey.value);
    if (found) return found;
  }
  return trendItems.value[trendItems.value.length - 1];
});

const topTransactions = computed<Transaction[]>(() => {
  const list = statsData.value?.top_transactions || [];
  const filtered = highlightedCatId.value
    ? list.filter((tx) => tx.category_id === highlightedCatId.value)
    : list;
  return filtered.slice(0, 5);
});

const highlightedCatName = computed(() => {
  if (!highlightedCatId.value) return '';
  return (
    categoryBreakdown.value.find((c) => c.category_id === highlightedCatId.value)
      ?.category_name || ''
  );
});

const fallbackInsightText = computed(() => {
  const topCat = categoryBreakdown.value[0];
  if (!topCat) {
    return '当前周期暂无记录，记一笔即可由七猫 AI 生成专属消费洞察。';
  }
  const typeText = store.statsType === 'income' ? '收入' : '支出';
  return `当前${typeText}占比最高的是「${topCat.category_name}」（占 ${Math.round(topCat.percentage)}%），合计 ${formatMoney(topCat.amount, { privacy: store.privacyHidden })}。`;
});

async function loadAiInsights() {
  const bid = store.activeBookId;
  if (!bid) {
    insights.value = null;
    return;
  }
  loadingInsight.value = true;
  try {
    insights.value = await api.getAiInsights({
      book_id: bid,
      period: store.statsPeriod,
      type: store.statsType,
      seed: insightSeed.value,
    });
  } catch {
    insights.value = null;
  } finally {
    loadingInsight.value = false;
  }
}

async function handleChangePeriod(period: StatsPeriod) {
  if (loadingStats.value) return;
  highlightedCatId.value = null;
  selectedBarKey.value = null;
  loadingStats.value = true;
  try {
    await store.refreshStats(period, store.statsType);
    await loadAiInsights();
  } finally {
    loadingStats.value = false;
  }
}

async function handleChangeType(type: 'expense' | 'income') {
  if (loadingStats.value || store.statsType === type) return;
  highlightedCatId.value = null;
  selectedBarKey.value = null;
  loadingStats.value = true;
  try {
    await store.refreshStats(store.statsPeriod, type);
    await loadAiInsights();
  } finally {
    loadingStats.value = false;
  }
}

function refreshInsightSeed() {
  insightSeed.value += 1;
  void loadAiInsights();
}

function onSelectCategory(catId: string | null) {
  highlightedCatId.value = catId;
}

function toggleCategoryFilter(catId: string) {
  highlightedCatId.value = highlightedCatId.value === catId ? null : catId;
}

function onSelectBar(item: TrendBarItem) {
  selectedBarKey.value = item.key;
}

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

watch(
  () => [store.activeBookId, store.currentYear, store.currentMonth, store.stats?.total_expense, store.stats?.total_income],
  () => {
    if (trendItems.value.length > 0 && !trendItems.value.some((it) => it.key === selectedBarKey.value)) {
      selectedBarKey.value = trendItems.value[trendItems.value.length - 1].key;
    }
    void loadAiInsights();
  },
  { immediate: true }
);
</script>

<template>
  <div class="space-y-3.5">
    <!-- 周期切换胶囊 -->
    <div class="p-1 rounded-2xl bg-[#ECEEF2] grid grid-cols-3 text-xs font-semibold">
      <button
        v-for="p in PERIOD_OPTIONS"
        :key="p.id"
        type="button"
        class="py-1.5 rounded-xl transition"
        :class="
          store.statsPeriod === p.id
            ? 'bg-white text-[#18181B] shadow-xs'
            : 'text-[#64748B] hover:text-[#18181B]'
        "
        @click="handleChangePeriod(p.id)"
      >
        {{ p.label }}
      </button>
    </div>

    <p class="text-center text-xs font-medium text-[#64748B] tabular-nums">
      {{ periodLabel }}
    </p>

    <!-- 3 列核心指标卡 -->
    <section class="app-card p-3.5 space-y-3">
      <div class="grid grid-cols-3 divide-x divide-[#ECEEF2] text-center">
        <div class="px-1">
          <p class="text-[11px] text-[#64748B]">
            {{ store.statsType === 'income' ? '周期总收入' : '周期总支出' }}
          </p>
          <p
            class="text-[14px] font-bold tabular-nums mt-1 truncate"
            :class="store.statsType === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
          >
            {{ formatMoney(activeTotal, { privacy: store.privacyHidden }) }}
          </p>
        </div>

        <div class="px-1">
          <p class="text-[11px] text-[#64748B]">
            {{ store.statsType === 'income' ? '日均入账' : '日均消费' }}
          </p>
          <p class="text-[14px] font-bold text-[#18181B] tabular-nums mt-1 truncate">
            {{ formatMoney(dailyAverage, { privacy: store.privacyHidden }) }}
          </p>
        </div>

        <div class="px-1">
          <p class="text-[11px] text-[#64748B]">笔数 / 结余率</p>
          <p class="text-[14px] font-bold text-[#DE8A1E] tabular-nums mt-1 truncate">
            {{ totalTxCount }} 笔
            <span class="text-[10px] font-semibold text-[#64748B]">
              · {{ Math.round(statsData?.savings_rate || 0) }}%
            </span>
          </p>
        </div>
      </div>

      <div
        class="pt-2.5 border-t border-[#F1F3F6] flex items-center justify-between text-[11px] text-[#64748B] tabular-nums"
      >
        <span>
          支出
          <strong class="text-[#D6553E] font-semibold">
            {{ formatMoney(totalExpense, { privacy: store.privacyHidden }) }}
          </strong>
        </span>
        <span>
          收入
          <strong class="text-[#3E8E63] font-semibold">
            {{ formatMoney(totalIncome, { privacy: store.privacyHidden }) }}
          </strong>
        </span>
        <span>
          结余
          <strong class="text-[#18181B] font-semibold">
            {{ formatMoney(netSavings, { privacy: store.privacyHidden }) }}
          </strong>
        </span>
      </div>
    </section>

    <!-- ✦ 七猫 AI 消费洞察卡片 -->
    <section class="rounded-2xl p-3.5 bg-[#FFF6E8] border border-[#F4DFBD] space-y-2.5">
      <div class="flex items-start gap-2.5">
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫 AI 洞察"
          class="w-8 h-8 rounded-full object-contain bg-white p-0.5 shrink-0 border border-white"
          @error="onMascotError"
        />
        <div class="flex-1 min-w-0 text-xs leading-relaxed text-[#5B3A13]">
          <div class="flex items-center justify-between gap-2 mb-1">
            <p class="font-bold text-[#3F270B]">✦ 七猫 AI 消费洞察</p>
            <div class="flex items-center gap-1.5 shrink-0">
              <button
                type="button"
                :disabled="loadingInsight"
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-white/85 text-[#DE8A1E] border border-[#F4DFBD] hover:bg-white transition disabled:opacity-60"
                @click="refreshInsightSeed"
              >
                {{ loadingInsight ? '生成中…' : '换一组' }}
              </button>
              <button
                type="button"
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#DE8A1E] text-white hover:bg-[#C67612] transition"
                @click="emit('open-ai')"
              >
                追问 AI
              </button>
            </div>
          </div>
          <p class="text-[11.5px] leading-relaxed">
            {{ insights?.summary || fallbackInsightText }}
          </p>
        </div>
      </div>

      <div v-if="insights?.tips?.length" class="space-y-1.5 pt-1">
        <div
          v-for="(tip, idx) in insights.tips.slice(0, 2)"
          :key="idx"
          class="rounded-xl bg-white/80 border border-[#F4DFBD]/70 px-3 py-2 flex items-start gap-2"
        >
          <span
            class="shrink-0 text-[10px] font-semibold px-1.5 py-0.5 rounded-md mt-0.5"
            :style="{
              color: tip.color || '#DE8A1E',
              backgroundColor: tip.bg || '#FFF6E8',
            }"
          >
            {{ tip.tag || '建议' }}
          </span>
          <div class="min-w-0">
            <p class="text-[11px] font-bold text-[#3F270B] truncate">{{ tip.title }}</p>
            <p class="text-[10.5px] text-[#6B4B21] leading-snug mt-0.5">{{ tip.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 分类占比环形图 + 分类明细排行进度条 -->
    <section class="app-card p-4">
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-sm font-bold text-[#18181B]">分类构成</h3>
        <div class="inline-flex p-0.5 rounded-full bg-[#EEF0F4] text-[11px] font-semibold">
          <button
            type="button"
            class="px-2.5 py-1 rounded-full transition"
            :class="
              store.statsType === 'expense'
                ? 'bg-[#DE8A1E] text-white shadow-xs'
                : 'text-[#64748B]'
            "
            @click="handleChangeType('expense')"
          >
            支出
          </button>
          <button
            type="button"
            class="px-2.5 py-1 rounded-full transition"
            :class="
              store.statsType === 'income'
                ? 'bg-[#DE8A1E] text-white shadow-xs'
                : 'text-[#64748B]'
            "
            @click="handleChangeType('income')"
          >
            收入
          </button>
        </div>
      </div>

      <p
        v-if="categoryBreakdown.length === 0"
        class="text-center text-xs text-[#94A3B8] py-8"
      >
        该周期暂无{{ store.statsType === 'expense' ? '支出' : '收入' }}数据
      </p>

      <template v-else>
        <DonutChart
          :items="donutItems"
          :total="activeTotal"
          :center-label="store.statsType === 'expense' ? '总支出' : '总收入'"
          :highlighted-id="highlightedCatId"
          :privacy="store.privacyHidden"
          @select="onSelectCategory"
        />

        <!-- 分类明细排行进度条 -->
        <div class="mt-4 pt-3.5 border-t border-[#F1F3F6] space-y-2.5">
          <button
            v-for="cat in categoryBreakdown"
            :key="cat.category_id"
            type="button"
            class="w-full text-left p-2 -mx-2 rounded-xl transition"
            :class="
              highlightedCatId === cat.category_id
                ? 'bg-[#FFF6E8]/70 ring-1 ring-[#DE8A1E]/40'
                : 'hover:bg-[#F8F9FB]'
            "
            @click="toggleCategoryFilter(cat.category_id)"
          >
            <div class="flex items-center justify-between gap-2 mb-1.5">
              <div class="flex items-center gap-2 min-w-0">
                <CategoryBadge
                  :icon="cat.category_icon || 'tag'"
                  :color="cat.category_color || '#DE8A1E'"
                  :bg-color="cat.bg_color || '#FFF6E8'"
                  size="xs"
                />
                <span class="text-xs font-semibold text-[#18181B] truncate">
                  {{ cat.category_name }}
                </span>
                <span class="text-[10px] text-[#94A3B8] shrink-0">
                  {{ cat.count }} 笔
                </span>
              </div>
              <div class="flex items-center gap-2 shrink-0 tabular-nums">
                <span class="text-[11px] text-[#64748B]">
                  {{ Math.round(cat.percentage || 0) }}%
                </span>
                <span class="text-xs font-bold text-[#18181B]">
                  {{ formatMoney(cat.amount, { privacy: store.privacyHidden }) }}
                </span>
              </div>
            </div>
            <div class="h-1.5 rounded-full bg-[#ECEEF2] overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-300"
                :style="{
                  width: `${Math.min(100, Math.max(2, Math.round(cat.percentage || 0)))}%`,
                  backgroundColor: cat.category_color || '#DE8A1E',
                }"
              />
            </div>
          </button>
        </div>
      </template>
    </section>

    <!-- 趋势柱状图 -->
    <section class="app-card p-4">
      <div class="flex items-center justify-between mb-2">
        <h3 class="text-sm font-bold text-[#18181B]">
          {{ store.statsPeriod === 'year' ? '月度收支趋势' : '近期每日趋势' }}
        </h3>
        <span
          v-if="activeBarItem"
          class="text-[11px] font-semibold px-2.5 py-1 rounded-full bg-[#F5F6F8] text-[#18181B] tabular-nums"
        >
          {{ activeBarItem.label }} ·
          {{ formatMoney(activeBarItem.value, { privacy: store.privacyHidden }) }}
        </span>
      </div>

      <p
        v-if="trendItems.length === 0"
        class="text-center text-xs text-[#94A3B8] py-8"
      >
        暂无趋势数据
      </p>

      <TrendBarChart
        v-else
        :items="trendItems"
        :selected-key="activeBarItem?.key || null"
        :accent-color="store.statsType === 'income' ? '#3E8E63' : '#D6553E'"
        mode="mobile"
        :privacy="store.privacyHidden"
        @select="onSelectBar"
      />
    </section>

    <!-- 单笔金额排行 Top 5 -->
    <section class="app-card p-4">
      <div class="flex items-center justify-between mb-2.5">
        <h3 class="text-sm font-bold text-[#18181B]">
          单笔{{ store.statsType === 'expense' ? '支出' : '收入' }}排行 Top 5
        </h3>
        <button
          v-if="highlightedCatId"
          type="button"
          class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
          @click="highlightedCatId = null"
        >
          仅看「{{ highlightedCatName }}」· 清除
        </button>
      </div>

      <p v-if="topTransactions.length === 0" class="text-xs text-[#94A3B8] py-4 text-center">
        暂无排行数据
      </p>

      <div v-else class="space-y-2">
        <button
          v-for="(tx, idx) in topTransactions"
          :key="tx.id"
          type="button"
          class="w-full flex items-center justify-between gap-2.5 text-left hover:bg-[#F8F9FB] p-1.5 -mx-1.5 rounded-xl transition"
          @click="emit('select-tx', tx)"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <span
              class="w-5 text-center text-xs font-bold tabular-nums shrink-0"
              :class="idx === 0 ? 'text-[#D6553E]' : idx === 1 ? 'text-[#DE8A1E]' : 'text-[#94A3B8]'"
            >
              {{ idx + 1 }}
            </span>
            <CategoryBadge
              :icon="tx.category_icon || 'tag'"
              :color="tx.category_color || (tx.type === 'income' ? '#3E8E63' : '#64748B')"
              :bg-color="tx.category_bg_color || (tx.type === 'income' ? '#ECF6F0' : '#F1F5F9')"
              size="sm"
            />
            <div class="min-w-0">
              <p class="text-xs font-semibold text-[#18181B] truncate">
                {{ tx.title || tx.category_name || '未命名账单' }}
              </p>
              <p class="text-[10px] text-[#94A3B8] truncate">
                {{ tx.date ? tx.date.slice(5) : '' }} · {{ tx.category_name || '默认分类' }}
              </p>
            </div>
          </div>

          <span
            class="text-xs font-bold tabular-nums shrink-0"
            :class="tx.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
          >
            {{
              formatMoney(tx.amount, {
                privacy: store.privacyHidden,
                showSign: true,
                type: tx.type,
              })
            }}
          </span>
        </button>
      </div>
    </section>
  </div>
</template>
