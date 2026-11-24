<script setup lang="ts">
import { computed } from 'vue';
import type { DailyReportItem } from '@/types';
import { centsToYuan, formatMoney, todayIso } from '@/utils/format';

const props = withDefaults(
  defineProps<{
    year: number;
    month: number; // 1-12
    dailyItems: DailyReportItem[];
    selectedDay?: number | null;
    privacy?: boolean;
  }>(),
  {
    selectedDay: null,
    privacy: false,
  }
);

const emit = defineEmits<{
  'select-day': [day: number, isoDate: string];
  'change-month': [year: number, month: number];
  'go-today': [];
}>();

const WEEKDAYS = ['一', '二', '三', '四', '五', '六', '日'];

const monthPrefix = computed(
  () => `${props.year}-${String(props.month).padStart(2, '0')}-`
);

const leadingBlanks = computed(() => {
  const firstDay = new Date(props.year, props.month - 1, 1).getDay();
  return (firstDay + 6) % 7; // 周一为 0，周日为 6
});

const daysInMonth = computed(() =>
  new Date(props.year, props.month, 0).getDate()
);

const dayMap = computed(() => {
  const map: Record<number, { expense: number; income: number }> = {};
  for (const item of props.dailyItems || []) {
    if (!item.date || !item.date.startsWith(monthPrefix.value)) continue;
    const d = Number(item.date.slice(8, 10));
    if (!Number.isFinite(d) || d <= 0) continue;
    if (!map[d]) map[d] = { expense: 0, income: 0 };
    map[d].expense += Number(item.expense || 0);
    map[d].income += Number(item.income || 0);
  }
  return map;
});

const monthTotals = computed(() => {
  let expense = 0;
  let income = 0;
  for (const val of Object.values(dayMap.value)) {
    expense += val.expense;
    income += val.income;
  }
  return { expense, income };
});

const todayStr = todayIso();

interface CalendarCell {
  day: number;
  isoDate: string;
  expense: number;
  income: number;
  isToday: boolean;
  isSelected: boolean;
  bgClass: string;
  expenseLabel: string;
}

const cells = computed<CalendarCell[]>(() => {
  const result: CalendarCell[] = [];
  for (let d = 1; d <= daysInMonth.value; d++) {
    const isoDate = `${monthPrefix.value}${String(d).padStart(2, '0')}`;
    const info = dayMap.value[d] || { expense: 0, income: 0 };
    const expYuan = centsToYuan(info.expense);

    let bgClass = 'bg-[#F7F8FA] text-[#27272A] hover:bg-[#ECEEF2]';
    if (info.expense > 0) {
      if (expYuan >= 500) bgClass = 'bg-[#F5C4BC] text-[#8A281A]';
      else if (expYuan >= 150) bgClass = 'bg-[#FADCD6] text-[#A13424]';
      else bgClass = 'bg-[#FDF0EE] text-[#B84330]';
    } else if (info.income > 0) {
      bgClass = 'bg-[#EAF5EE] text-[#3E8E63]';
    }

    let expenseLabel = '';
    if (info.expense > 0) {
      if (props.privacy) {
        expenseLabel = '-**';
      } else if (expYuan >= 1000) {
        expenseLabel = `-${(expYuan / 1000).toFixed(1)}k`;
      } else {
        expenseLabel = `-${Math.round(expYuan)}`;
      }
    }

    result.push({
      day: d,
      isoDate,
      expense: info.expense,
      income: info.income,
      isToday: isoDate === todayStr,
      isSelected: props.selectedDay === d,
      bgClass,
      expenseLabel,
    });
  }
  return result;
});

function prevMonth() {
  if (props.month === 1) {
    emit('change-month', props.year - 1, 12);
  } else {
    emit('change-month', props.year, props.month - 1);
  }
}

function nextMonth() {
  if (props.month === 12) {
    emit('change-month', props.year + 1, 1);
  } else {
    emit('change-month', props.year, props.month + 1);
  }
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between gap-2 mb-3">
      <div>
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            class="w-6 h-6 rounded-full bg-[#F5F6F8] hover:bg-[#ECEEF2] text-[#475569] flex items-center justify-center transition"
            title="上个月"
            @click="prevMonth"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          </button>
          <h2 class="text-base font-bold text-[#18181B]">
            {{ year }}年{{ month }}月 · 收支日历
          </h2>
          <button
            type="button"
            class="w-6 h-6 rounded-full bg-[#F5F6F8] hover:bg-[#ECEEF2] text-[#475569] flex items-center justify-center transition"
            title="下个月"
            @click="nextMonth"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M9 18l6-6-6-6" />
            </svg>
          </button>
        </div>
        <p class="text-[11px] text-[#64748B] tabular-nums mt-0.5">
          月支 {{ formatMoney(monthTotals.expense, { privacy }) }} · 月收
          {{ formatMoney(monthTotals.income, { privacy }) }}
        </p>
      </div>
      <button
        type="button"
        class="text-[11px] font-semibold px-2.5 py-1 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition shrink-0"
        @click="emit('go-today')"
      >
        回今天
      </button>
    </div>

    <!-- Weekday Row -->
    <div class="grid grid-cols-7 gap-1 text-center text-[10px] font-semibold text-[#94A3B8] mb-1.5">
      <div v-for="w in WEEKDAYS" :key="w">{{ w }}</div>
    </div>

    <!-- Calendar Grid -->
    <div class="grid grid-cols-7 gap-1">
      <div v-for="blank in leadingBlanks" :key="'b-' + blank" class="h-12" />
      <button
        v-for="cell in cells"
        :key="cell.day"
        type="button"
        class="h-12 rounded-xl flex flex-col items-center justify-center relative transition"
        :class="[
          cell.bgClass,
          cell.isSelected ? 'ring-2 ring-[#DE8A1E] ring-offset-1' : '',
        ]"
        @click="emit('select-day', cell.day, cell.isoDate)"
      >
        <span
          class="text-xs font-semibold leading-none"
          :class="{ 'underline decoration-[#DE8A1E] decoration-2': cell.isToday }"
        >
          {{ cell.day }}
        </span>

        <span
          v-if="cell.expense > 0"
          class="text-[9px] tabular-nums mt-1 font-medium leading-none"
        >
          {{ cell.expenseLabel }}
        </span>
        <span
          v-else-if="cell.income > 0"
          class="text-[9px] tabular-nums mt-1 font-medium leading-none text-[#3E8E63]"
        >
          {{ privacy ? '+**' : '+收' }}
        </span>
        <span v-else class="text-[9px] opacity-0 mt-1">-</span>

        <span
          v-if="cell.income > 0 && cell.expense > 0"
          class="w-1.5 h-1.5 rounded-full bg-[#3E8E63] absolute top-1 right-1"
        />
      </button>
    </div>

    <!-- Heatmap Legend -->
    <div class="mt-3 pt-2.5 border-t border-[#ECEEF2] flex items-center justify-between text-[10px] text-[#64748B]">
      <div class="flex items-center gap-1.5">
        <span class="w-2.5 h-2.5 rounded bg-[#EAF5EE] border border-[#C6E5D2]" />
        <span>有收入</span>
      </div>
      <div class="flex items-center gap-1">
        <span>低支出</span>
        <span class="w-2.5 h-2.5 rounded bg-[#FDF0EE]" />
        <span class="w-2.5 h-2.5 rounded bg-[#FADCD6]" />
        <span class="w-2.5 h-2.5 rounded bg-[#F5C4BC]" />
        <span>高支出</span>
      </div>
    </div>
  </div>
</template>
