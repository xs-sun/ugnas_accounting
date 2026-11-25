<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { DEFAULT_MASCOT, MASCOT_FALLBACK, resolveAvatarUrl } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney } from '@/utils/format';
import type { AiInsightsResponse, StatsPeriod, Transaction } from '@/types';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import DonutChart from '@/components/shared/DonutChart.vue';
import type { DonutChartItem } from '@/components/shared/DonutChart.vue';
import TrendBarChart from '@/components/shared/TrendBarChart.vue';
import type { TrendBarItem } from '@/components/shared/TrendBarChart.vue';

const emit = defineEmits<{
  'edit-tx': [tx: Transaction];
  'open-record': [opts?: { mode?: 'ai' | 'manual'; prompt?: string }];
}>();

const store = useAppStore();

const aiSeed = ref(0);
const insightsLoading = ref(false);
const insights = ref<AiInsightsResponse | null>(null);
const highlightedCatId = ref<string | null>(null);
const selectedBarKey = ref<string | null>(null);

const PERIOD_OPTIONS: Array<{ id: StatsPeriod; label: string }> = [
  { id: 'week', label: '本周' },
  { id: 'month', label: '本月' },
  { id: 'year', label: '全年' },
];

const periodLabel = computed(() => {
  if (store.statsPeriod === 'week') return '本周';
  if (store.statsPeriod === 'year') return '全年';
  return '本月';
});

async function loadInsights() {
  if (!store.activeBookId) {
    insights.value = null;
    return;
  }
  insightsLoading.value = true;
  try {
    insights.value = await api.getAiInsights({
      book_id: store.activeBookId,
      period: store.statsPeriod,
      type: store.statsType,
      seed: aiSeed.value,
    });
  } catch {
    insights.value = null;
  } finally {
    insightsLoading.value = false;
  }
}

watch(
  () => [
    store.activeBookId,
    store.statsPeriod,
    store.statsType,
    store.currentYear,
    store.currentMonth,
    store.transactionsTotal,
  ] as const,
  () => {
    loadInsights();
  },
  { immediate: true }
);

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

async function switchPeriod(period: StatsPeriod) {
  if (store.statsPeriod === period) return;
  highlightedCatId.value = null;
  selectedBarKey.value = null;
  await store.refreshStats(period, store.statsType);
}

async function switchType(type: 'expense' | 'income') {
  if (store.statsType === type) return;
  highlightedCatId.value = null;
  selectedBarKey.value = null;
  await store.refreshStats(store.statsPeriod, type);
}

function refreshAiInsights() {
  aiSeed.value += 1;
  loadInsights();
  store.showToast('已刷新 AI 消费诊断与建议', 'info');
}

function askAiFollowup() {
  emit('open-record', {
    mode: 'ai',
    prompt: `结合我${periodLabel.value}的收支数据，给出更具体的省钱与预算优化方案`,
  });
}

const activeTotal = computed<number>(() => {
  if (!store.stats) return 0;
  return store.statsType === 'income'
    ? store.stats.total_income || 0
    : store.stats.total_expense || 0;
});

const categoryRows = computed(() => store.stats?.categories || []);

const memberRows = computed(() => store.stats?.members || []);
const highlightedMemberId = ref<string | null>(null);

const memberDonutItems = computed<DonutChartItem[]>(() =>
  memberRows.value.map((m) => ({
    category_id: m.member_id,
    category_name: m.member_name,
    category_color: m.member_color || '#DE8A1E',
    amount: m.amount,
    percentage: m.percentage,
  }))
);

const donutItems = computed<DonutChartItem[]>(() =>
  categoryRows.value.map((c) => ({
    category_id: c.category_id,
    category_name: c.category_name,
    category_color: c.category_color || (store.statsType === 'income' ? '#3E8E63' : '#DE8A1E'),
    amount: c.amount,
    percentage: c.percentage,
  }))
);

const trendItems = computed<TrendBarItem[]>(() =>
  (store.stats?.daily_series || []).map((item) => ({
    key: item.date || item.label,
    label: item.label || item.date.slice(5),
    value: store.statsType === 'income' ? item.income || 0 : item.expense || 0,
    rawDate: item.date,
  }))
);

const topTransactions = computed<Transaction[]>(
  () => (store.stats?.top_transactions || []).slice(0, 5)
);

function handleSelectBar(item: TrendBarItem) {
  selectedBarKey.value = selectedBarKey.value === item.key ? null : item.key;
}
</script>

<template>
  <div class="space-y-5">
    <!-- 周期与收支切换栏 -->
    <section class="flex flex-wrap items-center justify-between gap-3">
      <div class="p-1 rounded-2xl bg-white border border-[#EAECEF] inline-flex text-xs font-semibold">
        <button
          v-for="p in PERIOD_OPTIONS"
          :key="p.id"
          type="button"
          class="px-4 py-1.5 rounded-xl transition"
          :class="
            store.statsPeriod === p.id
              ? 'bg-[#FFF3DC] text-[#B8680E] font-bold'
              : 'text-[#64748B] hover:text-[#18181B]'
          "
          @click="switchPeriod(p.id)"
        >
          {{ p.label }}
        </button>
      </div>

      <div class="p-1 rounded-2xl bg-white border border-[#EAECEF] inline-flex text-xs font-semibold">
        <button
          type="button"
          class="px-4 py-1.5 rounded-xl transition"
          :class="
            store.statsType === 'expense'
              ? 'bg-[#D6553E] text-white'
              : 'text-[#64748B] hover:text-[#18181B]'
          "
          @click="switchType('expense')"
        >
          支出分析
        </button>
        <button
          type="button"
          class="px-4 py-1.5 rounded-xl transition"
          :class="
            store.statsType === 'income'
              ? 'bg-[#3E8E63] text-white'
              : 'text-[#64748B] hover:text-[#18181B]'
          "
          @click="switchType('income')"
        >
          收入分析
        </button>
      </div>
    </section>

    <!-- ✦ 七猫 AI 财务诊断与建议 3 卡片横幅 -->
    <section class="pc-card p-5 bg-gradient-to-br from-[#FFFDF9] via-white to-[#FFF8EC] border-[#F2DFBE]">
      <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
        <div class="flex items-center gap-3 min-w-0">
          <img
            :src="DEFAULT_MASCOT"
            alt="七猫AI"
            class="w-10 h-10 rounded-2xl object-contain bg-white p-1 border border-[#F2DFBE] shadow-xs shrink-0"
            @error="onMascotError"
          />
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-sm font-bold text-[#18181B]">✦ 七猫 AI 财务诊断与建议</h2>
              <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#FFF3DC] text-[#B8680E]">
                基于{{ periodLabel }}真实流水
              </span>
            </div>
            <p class="text-[11px] text-[#64748B] mt-0.5 truncate">
              {{ insights?.summary || '自动分析收支结构、异常波动与下周期优化空间' }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            type="button"
            :disabled="insightsLoading"
            class="px-3 py-1.5 rounded-xl bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-xs font-semibold text-[#475569] hover:text-[#DE8A1E] disabled:opacity-60 transition"
            @click="refreshAiInsights"
          >
            {{ insightsLoading ? '分析中…' : '换一组洞察' }}
          </button>
          <button
            type="button"
            class="px-3.5 py-1.5 rounded-xl bg-[#FFF3DC] border border-[#DE8A1E]/40 text-xs font-semibold text-[#B8680E] hover:bg-[#FBEBD1] transition"
            @click="askAiFollowup"
          >
            ✦ 追问 AI 省钱方案
          </button>
        </div>
      </div>

      <div v-if="insights?.tips?.length" class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div
          v-for="(tip, idx) in insights.tips"
          :key="idx"
          class="p-3.5 rounded-2xl bg-white/95 border border-[#EAECEF] space-y-1.5"
        >
          <span
            class="inline-block text-[10px] font-bold px-2 py-0.5 rounded-full"
            :style="{ backgroundColor: tip.bg || '#FFF6E8', color: tip.color || '#DE8A1E' }"
          >
            {{ tip.tag }}
          </span>
          <p class="text-xs font-bold text-[#18181B] leading-snug">{{ tip.title }}</p>
          <p class="text-[11px] text-[#64748B] leading-relaxed">{{ tip.desc }}</p>
        </div>
      </div>
      <div v-else class="py-6 text-center text-xs text-[#94A3B8]">
        {{ insightsLoading ? 'AI 正在生成洞察建议…' : '记入更多收支后即可解锁定制诊断建议' }}
      </div>
    </section>

    <!-- 3 指标卡 -->
    <section class="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">{{ periodLabel }}总支出</p>
        <p class="text-2xl font-bold text-[#D6553E] tabular-nums mt-1.5">
          {{ formatMoney(store.stats?.total_expense ?? 0, { privacy: store.privacyHidden }) }}
        </p>
      </div>
      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">{{ periodLabel }}总收入</p>
        <p class="text-2xl font-bold text-[#3E8E63] tabular-nums mt-1.5">
          {{ formatMoney(store.stats?.total_income ?? 0, { privacy: store.privacyHidden }) }}
        </p>
      </div>
      <div class="pc-card p-5">
        <div class="flex items-center justify-between">
          <p class="text-xs text-[#64748B]">{{ periodLabel }}净结余</p>
          <span class="text-[11px] font-semibold text-[#DE8A1E] tabular-nums">
            储蓄率 {{ Math.round(store.stats?.savings_rate ?? 0) }}%
          </span>
        </div>
        <p
          class="text-2xl font-bold tabular-nums mt-1.5"
          :class="(store.stats?.net_savings ?? 0) < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
        >
          {{ formatMoney(store.stats?.net_savings ?? 0, { privacy: store.privacyHidden }) }}
        </p>
      </div>
    </section>

    <!-- 分类占比环形图与分类排行 -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <div class="pc-card p-5 flex flex-col justify-between space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">
            {{ store.statsType === 'expense' ? '支出' : '收入' }}分类占比
          </h2>
          <span class="text-xs font-bold tabular-nums text-[#18181B]">
            合计 {{ formatMoney(activeTotal, { privacy: store.privacyHidden }) }}
          </span>
        </div>

        <div v-if="donutItems.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
          该周期暂无{{ store.statsType === 'expense' ? '支出' : '收入' }}数据
        </div>
        <DonutChart
          v-else
          :items="donutItems"
          :total="activeTotal"
          :center-label="store.statsType === 'expense' ? '周期支出' : '周期收入'"
          :size="168"
          :highlighted-id="highlightedCatId"
          :privacy="store.privacyHidden"
          :max-legend-items="6"
          @select="highlightedCatId = $event"
        />
      </div>

      <div class="pc-card p-5 space-y-3.5">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">分类排行明细</h2>
          <span class="text-[11px] text-[#94A3B8]">共 {{ categoryRows.length }} 个分类</span>
        </div>

        <div v-if="categoryRows.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
          暂无分类排行数据
        </div>
        <div v-else class="space-y-3 max-h-64 overflow-y-auto thin-scroll pr-1">
          <button
            v-for="row in categoryRows"
            :key="row.category_id"
            type="button"
            class="w-full text-left space-y-1.5 p-2 -mx-1 rounded-xl transition"
            :class="highlightedCatId === row.category_id ? 'bg-[#FFF6E8]' : 'hover:bg-[#FAFBFD]'"
            @click="highlightedCatId = highlightedCatId === row.category_id ? null : row.category_id"
          >
            <div class="flex items-center justify-between text-xs">
              <div class="flex items-center gap-2 min-w-0">
                <CategoryBadge
                  :icon="row.category_icon || 'tag'"
                  :color="row.category_color || '#DE8A1E'"
                  :bg-color="row.bg_color || '#FFF6E8'"
                  size="xs"
                />
                <span class="font-semibold text-[#18181B] truncate">{{ row.category_name }}</span>
                <span class="text-[#94A3B8] tabular-nums">{{ Math.round(row.percentage) }}%</span>
              </div>
              <span class="font-bold tabular-nums text-[#18181B] shrink-0">
                {{ formatMoney(row.amount, { privacy: store.privacyHidden }) }}
              </span>
            </div>
            <div class="h-2 rounded-full bg-[#F1F3F6] overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-300"
                :style="{
                  width: `${Math.min(100, Math.max(2, Math.round(row.percentage || 0)))}%`,
                  backgroundColor: row.category_color || '#DE8A1E',
                }"
              />
            </div>
          </button>
        </div>
      </div>
    </section>

    <!-- 成员占比环形图与成员排行 -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <div class="pc-card p-5 flex flex-col justify-between space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">
            {{ store.statsType === 'expense' ? '支出' : '收入' }}成员占比
          </h2>
          <span class="text-xs font-bold tabular-nums text-[#18181B]">
            合计 {{ formatMoney(activeTotal, { privacy: store.privacyHidden }) }}
          </span>
        </div>

        <div v-if="memberDonutItems.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
          该周期暂无{{ store.statsType === 'expense' ? '支出' : '收入' }}数据
        </div>
        <DonutChart
          v-else
          :items="memberDonutItems"
          :total="activeTotal"
          :center-label="store.statsType === 'expense' ? '周期支出' : '周期收入'"
          :size="168"
          :highlighted-id="highlightedMemberId"
          :privacy="store.privacyHidden"
          :max-legend-items="6"
          @select="highlightedMemberId = $event"
        />
      </div>

      <div class="pc-card p-5 space-y-3.5">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">成员排行明细</h2>
          <span class="text-[11px] text-[#94A3B8]">共 {{ memberRows.length }} 位成员</span>
        </div>

        <div v-if="memberRows.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
          暂无成员排行数据
        </div>
        <div v-else class="space-y-3 max-h-64 overflow-y-auto thin-scroll pr-1">
          <button
            v-for="row in memberRows"
            :key="row.member_id"
            type="button"
            class="w-full text-left space-y-1.5 p-2 -mx-1 rounded-xl transition"
            :class="highlightedMemberId === row.member_id ? 'bg-[#FFF6E8]' : 'hover:bg-[#FAFBFD]'"
            @click="highlightedMemberId = highlightedMemberId === row.member_id ? null : row.member_id"
          >
            <div class="flex items-center justify-between text-xs">
              <div class="flex items-center gap-2 min-w-0">
                <img
                  v-if="row.member_avatar"
                  :src="resolveAvatarUrl(row.member_avatar)"
                  :alt="row.member_name"
                  class="w-6 h-6 rounded-full object-cover bg-white border border-[#ECEEF2] shrink-0"
                />
                <span
                  v-else
                  class="w-6 h-6 rounded-full text-[10px] font-bold text-white flex items-center justify-center shrink-0"
                  :style="{ backgroundColor: row.member_color || '#DE8A1E' }"
                >
                  {{ (row.member_name || '?').slice(0, 1) }}
                </span>
                <span class="font-semibold text-[#18181B] truncate">{{ row.member_name }}</span>
                <span
                  v-if="row.is_virtual"
                  class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-[#EEF2FF] text-[#4F46E5] shrink-0"
                >
                  虚拟成员
                </span>
                <span class="text-[#94A3B8] tabular-nums">{{ row.count }} 笔</span>
                <span class="text-[#94A3B8] tabular-nums">{{ Math.round(row.percentage) }}%</span>
              </div>
              <span class="font-bold tabular-nums text-[#18181B] shrink-0">
                {{ formatMoney(row.amount, { privacy: store.privacyHidden }) }}
              </span>
            </div>
            <div class="h-2 rounded-full bg-[#F1F3F6] overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-300"
                :style="{
                  width: `${Math.min(100, Math.max(2, Math.round(row.percentage || 0)))}%`,
                  backgroundColor: row.member_color || '#DE8A1E',
                }"
              />
            </div>
          </button>
        </div>
      </div>
    </section>

    <!-- 趋势柱状图与单笔 Top 5 -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <div class="pc-card p-5 space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">
            {{ periodLabel }}{{ store.statsType === 'expense' ? '支出' : '收入' }}趋势
          </h2>
          <span class="text-[11px] text-[#94A3B8]">点击柱条可高亮对比</span>
        </div>

        <div v-if="trendItems.length === 0" class="h-44 flex items-center justify-center text-xs text-[#94A3B8]">
          暂无趋势数据
        </div>
        <TrendBarChart
          v-else
          mode="pc"
          :items="trendItems"
          :selected-key="selectedBarKey"
          :accent-color="store.statsType === 'income' ? '#3E8E63' : '#DE8A1E'"
          :privacy="store.privacyHidden"
          height-class="h-44"
          @select="handleSelectBar"
        />
      </div>

      <div class="pc-card p-5 space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">
            单笔{{ store.statsType === 'expense' ? '支出' : '收入' }} Top 5
          </h2>
          <span class="text-[11px] text-[#94A3B8]">点击可编辑修正</span>
        </div>

        <div v-if="topTransactions.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
          暂无排行明细
        </div>
        <div v-else class="divide-y divide-[#F1F3F6]">
          <button
            v-for="(tx, idx) in topTransactions"
            :key="tx.id"
            type="button"
            class="w-full py-2.5 flex items-center justify-between hover:bg-[#FAFBFD] rounded-xl px-2 -mx-2 transition text-left"
            @click="emit('edit-tx', tx)"
          >
            <div class="flex items-center gap-2.5 min-w-0">
              <span
                class="w-5 text-center text-xs font-bold shrink-0"
                :class="idx === 0 ? 'text-[#D6553E]' : 'text-[#94A3B8]'"
              >
                {{ idx + 1 }}
              </span>
              <CategoryBadge
                :icon="tx.category_icon || 'tag'"
                :color="tx.category_color || '#DE8A1E'"
                :bg-color="tx.category_bg_color || '#FFF6E8'"
                size="sm"
              />
              <div class="min-w-0">
                <p class="text-xs font-bold text-[#18181B] truncate">
                  {{ tx.title || tx.category_name || '未命名账单' }}
                </p>
                <p class="text-[10px] text-[#94A3B8] truncate">
                  {{ tx.date }} · {{ tx.category_name || '未分类' }}
                </p>
              </div>
            </div>
            <span
              class="text-xs font-bold tabular-nums shrink-0"
              :class="tx.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
            >
              {{ formatMoney(tx.amount, { privacy: store.privacyHidden }) }}
            </span>
          </button>
        </div>
      </div>
    </section>
  </div>
</template>
