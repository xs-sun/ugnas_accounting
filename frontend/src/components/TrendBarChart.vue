<script setup lang="ts">
import { computed } from 'vue';
import type { TrendMonthItem } from '../types';

const props = defineProps<{
  months: TrendMonthItem[];
}>();

const maxVal = computed(() => {
  let m = 10000;
  for (const it of props.months) {
    if (it.expense > m) m = it.expense;
    if (it.income > m) m = it.income;
  }
  return m;
});

const chartHeight = 140;
</script>

<template>
  <div class="w-full">
    <div v-if="months.length === 0" class="h-36 flex items-center justify-center text-xs text-[#9AA0AB]">
      暂无趋势数据
    </div>
    <div v-else class="space-y-2">
      <!-- Grid & Bars -->
      <div class="relative h-36 flex items-end justify-between px-2 pt-6 border-b border-[#EDEEF1]">
        <!-- Faint Background Grid Lines -->
        <div class="absolute inset-x-0 top-6 border-b border-dashed border-[#EDEEF1]/60 pointer-events-none"></div>
        <div class="absolute inset-x-0 top-20 border-b border-dashed border-[#EDEEF1]/60 pointer-events-none"></div>

        <!-- Month Columns -->
        <div
          v-for="item in months"
          :key="item.month"
          class="flex-1 flex flex-col items-center justify-end h-full group"
        >
          <div class="flex items-end space-x-1 mb-1">
            <!-- Expense Bar (Terracotta Orange #E5654E) -->
            <div
              class="w-2.5 bg-[#E5654E] rounded-t-[3px] transition-all duration-300 group-hover:brightness-105"
              :style="{
                height: `${Math.max(4, Math.round((item.expense / maxVal) * (chartHeight - 30)))}px`,
              }"
              :title="`支出: ¥${(item.expense / 100).toFixed(2)}`"
            ></div>

            <!-- Income Bar (Deep Turquoise #0E8C7F) -->
            <div
              class="w-2.5 bg-[#0E8C7F] rounded-t-[3px] transition-all duration-300 group-hover:brightness-105"
              :style="{
                height: `${Math.max(4, Math.round((item.income / maxVal) * (chartHeight - 30)))}px`,
              }"
              :title="`收入: ¥${(item.income / 100).toFixed(2)}`"
            ></div>
          </div>
          <span class="text-[11px] text-[#9AA0AB] font-mono whitespace-nowrap">
            {{ item.month.slice(5) }}月
          </span>
        </div>
      </div>

      <!-- Legend -->
      <div class="flex items-center justify-end space-x-4 text-xs text-[#5D6470] pt-1">
        <div class="flex items-center space-x-1.5">
          <span class="w-2.5 h-2.5 rounded-[2px] bg-[#E5654E]"></span>
          <span>支出</span>
        </div>
        <div class="flex items-center space-x-1.5">
          <span class="w-2.5 h-2.5 rounded-[2px] bg-[#0E8C7F]"></span>
          <span>收入</span>
        </div>
      </div>
    </div>
  </div>
</template>
