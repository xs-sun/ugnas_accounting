<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney, getWeekdayLabel, todayIso } from '@/utils/format';
import type { Transaction } from '@/types';
import CalendarHeatmap from '@/components/shared/CalendarHeatmap.vue';
import TransactionRow from '@/components/shared/TransactionRow.vue';

const emit = defineEmits<{
  'select-tx': [tx: Transaction];
  'add-for-date': [isoDate: string];
}>();

const store = useAppStore();

const now = new Date();
const selectedDay = ref<number>(now.getDate());
const monthTransactions = ref<Transaction[]>([]);
const loadingMonth = ref(false);

const monthPrefix = computed(
  () => `${store.currentYear}-${String(store.currentMonth).padStart(2, '0')}`
);

const selectedIsoDate = computed(
  () => `${monthPrefix.value}-${String(selectedDay.value).padStart(2, '0')}`
);

// 选中月份的起止日期
const monthRange = computed(() => {
  const daysInMonth = new Date(store.currentYear, store.currentMonth, 0).getDate();
  return {
    start_date: `${monthPrefix.value}-01`,
    end_date: `${monthPrefix.value}-${String(daysInMonth).padStart(2, '0')}`,
    daysInMonth,
  };
});

async function loadMonthTransactions() {
  const bid = store.activeBookId;
  if (!bid) {
    monthTransactions.value = [];
    return;
  }
  loadingMonth.value = true;
  try {
    const res = await api.listTransactions({
      book_id: bid,
      start_date: monthRange.value.start_date,
      end_date: monthRange.value.end_date,
      page: 1,
      page_size: 200,
    });
    monthTransactions.value = res.items || [];
  } catch {
    // 若网络请求失败则回退从 store.transactions 筛选当前月份
    monthTransactions.value = store.transactions.filter((tx) =>
      tx.date?.startsWith(monthPrefix.value)
    );
  } finally {
    loadingMonth.value = false;
  }
}

watch(
  () => [
    store.activeBookId,
    store.currentYear,
    store.currentMonth,
    store.transactions.length,
    store.transactions[0]?.id,
    store.transactions[0]?.updated_at,
  ],
  () => {
    void loadMonthTransactions();
  },
  { immediate: true }
);

const selectedDayTransactions = computed(() =>
  monthTransactions.value
    .filter((tx) => (tx.date || '').slice(0, 10) === selectedIsoDate.value)
    .sort((a, b) => (b.tx_time || '').localeCompare(a.tx_time || ''))
);

const selectedDayTotals = computed(() => {
  let expense = 0;
  let income = 0;
  for (const tx of selectedDayTransactions.value) {
    if (tx.type === 'expense') expense += Number(tx.amount || 0);
    else if (tx.type === 'income') income += Number(tx.amount || 0);
  }
  return { expense, income };
});

function onSelectDay(day: number) {
  selectedDay.value = day;
}

async function onChangeMonth(year: number, month: number) {
  await store.setCurrentMonth(year, month);
  const current = new Date();
  if (year === current.getFullYear() && month === current.getMonth() + 1) {
    selectedDay.value = current.getDate();
  } else {
    const daysInNext = new Date(year, month, 0).getDate();
    if (selectedDay.value > daysInNext) {
      selectedDay.value = daysInNext;
    } else {
      selectedDay.value = 1;
    }
  }
}

async function onGoToday() {
  const current = new Date();
  const y = current.getFullYear();
  const m = current.getMonth() + 1;
  selectedDay.value = current.getDate();
  if (store.currentYear !== y || store.currentMonth !== m) {
    await store.setCurrentMonth(y, m);
  } else {
    await store.refreshDailyReport(y, m);
  }
}

function handleAddForSelectedDate() {
  emit('add-for-date', selectedIsoDate.value || todayIso());
}
</script>

<template>
  <div class="space-y-3.5">
    <!-- 收支热力日历卡片 -->
    <section class="app-card p-4">
      <CalendarHeatmap
        :year="store.currentYear"
        :month="store.currentMonth"
        :daily-items="store.dailyReport"
        :selected-day="selectedDay"
        :privacy="store.privacyHidden"
        @select-day="onSelectDay"
        @change-month="onChangeMonth"
        @go-today="onGoToday"
      />
    </section>

    <!-- 选中日期流水下钻卡片 -->
    <section class="app-card overflow-hidden">
      <div
        class="px-4 py-2.5 bg-[#F8F9FB] border-b border-[#ECEEF2] flex items-center justify-between gap-2 text-xs"
      >
        <span class="font-semibold text-[#18181B]">
          {{ getWeekdayLabel(selectedIsoDate) }}
        </span>
        <div class="flex items-center gap-2.5 shrink-0">
          <span class="text-[11px] text-[#64748B] tabular-nums">
            支 {{ formatMoney(selectedDayTotals.expense, { privacy: store.privacyHidden }) }} · 收
            {{ formatMoney(selectedDayTotals.income, { privacy: store.privacyHidden }) }}
          </span>
          <button
            type="button"
            class="text-[11px] font-semibold px-2.5 py-0.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition"
            @click="handleAddForSelectedDate"
          >
            + 补记
          </button>
        </div>
      </div>

      <div v-if="loadingMonth" class="p-6 text-center text-xs text-[#94A3B8]">
        正在加载当日明细…
      </div>

      <div v-else-if="selectedDayTransactions.length === 0" class="p-6 text-center">
        <p class="text-xs font-medium text-[#64748B]">这一天没有记账记录</p>
        <button
          type="button"
          class="mt-2.5 text-xs font-semibold px-3.5 py-1.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition"
          @click="handleAddForSelectedDate"
        >
          补记这天
        </button>
      </div>

      <div v-else class="divide-y divide-[#F1F3F6]">
        <TransactionRow
          v-for="tx in selectedDayTransactions"
          :key="tx.id"
          :tx="tx"
          :privacy="store.privacyHidden"
          @click="emit('select-tx', $event)"
        />
      </div>
    </section>
  </div>
</template>
