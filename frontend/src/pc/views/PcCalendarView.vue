<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney, getWeekdayLabel, todayIso } from '@/utils/format';
import type { Transaction } from '@/types';
import CalendarHeatmap from '@/components/shared/CalendarHeatmap.vue';
import TransactionRow from '@/components/shared/TransactionRow.vue';

const emit = defineEmits<{
  'edit-tx': [tx: Transaction];
  'open-record': [opts?: { mode?: 'ai' | 'manual'; defaultDate?: string }];
}>();

const store = useAppStore();

function resolveInitialDay(): number {
  const now = new Date();
  if (now.getFullYear() === store.currentYear && now.getMonth() + 1 === store.currentMonth) {
    return now.getDate();
  }
  return 1;
}

const selectedDay = ref<number>(resolveInitialDay());
const monthTransactions = ref<Transaction[]>([]);
const loadingDay = ref(false);

const monthPrefix = computed(
  () => `${store.currentYear}-${String(store.currentMonth).padStart(2, '0')}-`
);

const selectedIsoDate = computed(
  () => `${monthPrefix.value}${String(selectedDay.value).padStart(2, '0')}`
);

async function fetchMonthTransactions() {
  if (!store.activeBookId) {
    monthTransactions.value = [];
    return;
  }
  loadingDay.value = true;
  try {
    const lastDay = new Date(store.currentYear, store.currentMonth, 0).getDate();
    const res = await api.listTransactions({
      book_id: store.activeBookId,
      start_date: `${monthPrefix.value}01`,
      end_date: `${monthPrefix.value}${String(lastDay).padStart(2, '0')}`,
      page: 1,
      page_size: 500,
    });
    monthTransactions.value = res.items || [];
  } catch {
    monthTransactions.value = store.transactions.filter((t) =>
      (t.date || '').startsWith(monthPrefix.value)
    );
  } finally {
    loadingDay.value = false;
  }
}

watch(
  () => [
    store.activeBookId,
    store.currentYear,
    store.currentMonth,
    store.transactionsTotal,
    store.transactions,
  ] as const,
  () => {
    const maxDay = new Date(store.currentYear, store.currentMonth, 0).getDate();
    if (selectedDay.value > maxDay) {
      selectedDay.value = maxDay;
    }
    fetchMonthTransactions();
  },
  { immediate: true }
);

const monthTotals = computed(() => {
  let expense = 0;
  let income = 0;
  for (const item of store.dailyReport) {
    expense += item.expense || 0;
    income += item.income || 0;
  }
  return {
    expense,
    income,
    balance: income - expense,
  };
});

const dayTransactions = computed<Transaction[]>(() =>
  monthTransactions.value
    .filter((tx) => (tx.date || '').startsWith(selectedIsoDate.value))
    .sort((a, b) => (b.tx_time || '').localeCompare(a.tx_time || ''))
);

const dayTotals = computed(() => {
  let expense = 0;
  let income = 0;
  for (const tx of dayTransactions.value) {
    if (tx.type === 'expense') expense += tx.amount || 0;
    else if (tx.type === 'income') income += tx.amount || 0;
  }
  return { expense, income };
});

function handleSelectDay(day: number) {
  selectedDay.value = day;
}

async function handleChangeMonth(year: number, month: number) {
  selectedDay.value = 1;
  await store.setCurrentMonth(year, month);
}

async function handleGoToday() {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth() + 1;
  selectedDay.value = now.getDate();
  if (store.currentYear !== y || store.currentMonth !== m) {
    await store.setCurrentMonth(y, m);
  }
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
    <!-- 左侧：月度收支汇总与热力日历 -->
    <section class="lg:col-span-7 xl:col-span-8 space-y-4">
      <div class="grid grid-cols-3 gap-3">
        <div class="pc-card p-4">
          <p class="text-[11px] text-[#64748B]">当月支出</p>
          <p class="text-lg font-bold text-[#D6553E] tabular-nums mt-1">
            {{ formatMoney(monthTotals.expense, { privacy: store.privacyHidden }) }}
          </p>
        </div>
        <div class="pc-card p-4">
          <p class="text-[11px] text-[#64748B]">当月收入</p>
          <p class="text-lg font-bold text-[#3E8E63] tabular-nums mt-1">
            {{ formatMoney(monthTotals.income, { privacy: store.privacyHidden }) }}
          </p>
        </div>
        <div class="pc-card p-4">
          <p class="text-[11px] text-[#64748B]">当月结余</p>
          <p
            class="text-lg font-bold tabular-nums mt-1"
            :class="monthTotals.balance < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
          >
            {{ formatMoney(monthTotals.balance, { privacy: store.privacyHidden }) }}
          </p>
        </div>
      </div>

      <div class="pc-card p-5">
        <CalendarHeatmap
          :year="store.currentYear"
          :month="store.currentMonth"
          :daily-items="store.dailyReport"
          :selected-day="selectedDay"
          :privacy="store.privacyHidden"
          @select-day="handleSelectDay"
          @change-month="handleChangeMonth"
          @go-today="handleGoToday"
        />
      </div>
    </section>

    <!-- 右侧：选中日期流水明细 -->
    <section class="lg:col-span-5 xl:col-span-4 pc-card overflow-hidden flex flex-col">
      <div class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between gap-2 bg-white">
        <div class="min-w-0">
          <h3 class="text-sm font-bold text-[#18181B] truncate">
            {{ getWeekdayLabel(selectedIsoDate) }}
          </h3>
          <p class="text-[11px] text-[#64748B] tabular-nums mt-0.5">
            支 {{ formatMoney(dayTotals.expense, { privacy: store.privacyHidden }) }}
            · 收 {{ formatMoney(dayTotals.income, { privacy: store.privacyHidden }) }}
          </p>
        </div>

        <button
          type="button"
          class="shrink-0 px-3 py-1.5 rounded-xl bg-[#FFF6E8] hover:bg-[#FBEBD1] text-[#DE8A1E] text-xs font-semibold transition"
          @click="emit('open-record', { mode: 'manual', defaultDate: selectedIsoDate || todayIso() })"
        >
          + 补记这天
        </button>
      </div>

      <div v-if="loadingDay" class="py-12 text-center text-xs text-[#94A3B8]">
        正在加载当日明细…
      </div>
      <div v-else-if="dayTransactions.length === 0" class="py-14 px-4 text-center space-y-2.5">
        <p class="text-xs text-[#94A3B8]">{{ selectedIsoDate }} 暂无记账记录</p>
        <button
          type="button"
          class="px-4 py-1.5 rounded-full bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#475569] text-xs font-semibold transition"
          @click="emit('open-record', { mode: 'manual', defaultDate: selectedIsoDate })"
        >
          补记一笔
        </button>
      </div>
      <div v-else class="divide-y divide-[#F1F3F6] max-h-[540px] overflow-y-auto thin-scroll">
        <TransactionRow
          v-for="tx in dayTransactions"
          :key="tx.id"
          :tx="tx"
          :privacy="store.privacyHidden"
          @click="emit('edit-tx', $event)"
        />
      </div>
    </section>
  </div>
</template>
