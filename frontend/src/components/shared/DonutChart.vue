<script setup lang="ts">
import { computed } from 'vue';
import { formatMoney } from '@/utils/format';

export interface DonutChartItem {
  category_id: string;
  category_name: string;
  category_color: string;
  amount: number; // in cents
  percentage: number;
}

const props = withDefaults(
  defineProps<{
    items: DonutChartItem[];
    total: number; // in cents
    centerLabel?: string;
    size?: number;
    highlightedId?: string | null;
    privacy?: boolean;
    showLegend?: boolean;
    maxLegendItems?: number;
  }>(),
  {
    centerLabel: '总支出',
    size: 140,
    highlightedId: null,
    privacy: false,
    showLegend: true,
    maxLegendItems: 5,
  }
);

const emit = defineEmits<{
  select: [categoryId: string | null];
}>();

const RADIUS = 52;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

const segments = computed(() => {
  let offset = 0;
  const totalVal = props.total > 0 ? props.total : props.items.reduce((s, it) => s + it.amount, 0);
  return props.items.map((item) => {
    const len = totalVal > 0 ? (item.amount / totalVal) * CIRCUMFERENCE : 0;
    const seg = {
      item,
      dashArray: `${Math.max(0, len - 2)} ${CIRCUMFERENCE}`,
      dashOffset: -offset,
    };
    offset += len;
    return seg;
  });
});

const activeItem = computed<DonutChartItem | null>(() => {
  if (!props.highlightedId) return null;
  return props.items.find((it) => it.category_id === props.highlightedId) || null;
});

const legendItems = computed(() =>
  props.items.slice(0, props.maxLegendItems)
);

function handleSelect(categoryId: string) {
  emit('select', props.highlightedId === categoryId ? null : categoryId);
}
</script>

<template>
  <div class="flex items-center gap-4">
    <div
      class="relative shrink-0"
      :style="{ width: `${size}px`, height: `${size}px` }"
    >
      <svg viewBox="0 0 140 140" class="w-full h-full -rotate-90">
        <circle
          cx="70"
          cy="70"
          :r="RADIUS"
          fill="transparent"
          stroke="#ECEEF2"
          stroke-width="14"
        />
        <circle
          v-for="seg in segments"
          :key="seg.item.category_id"
          cx="70"
          cy="70"
          :r="RADIUS"
          fill="transparent"
          :stroke="seg.item.category_color || '#DE8A1E'"
          :stroke-width="highlightedId === seg.item.category_id ? 18 : 14"
          :stroke-dasharray="seg.dashArray"
          :stroke-dashoffset="seg.dashOffset"
          class="transition-all duration-300 cursor-pointer"
          @click="handleSelect(seg.item.category_id)"
        />
      </svg>

      <div
        class="absolute inset-0 flex flex-col items-center justify-center text-center px-3 pointer-events-none"
      >
        <span class="text-[10px] text-[#64748B] truncate max-w-full">
          {{ activeItem ? activeItem.category_name : centerLabel }}
        </span>
        <span class="text-xs font-bold text-[#18181B] tabular-nums mt-0.5">
          {{ formatMoney(activeItem ? activeItem.amount : total, { privacy }) }}
        </span>
        <span
          v-if="activeItem"
          class="text-[10px] font-semibold text-[#DE8A1E] tabular-nums"
        >
          {{ Math.round(activeItem.percentage) }}%
        </span>
      </div>
    </div>

    <div v-if="showLegend" class="flex-1 space-y-1.5 min-w-0">
      <button
        v-for="item in legendItems"
        :key="item.category_id"
        type="button"
        class="w-full flex items-center justify-between text-xs px-2 py-1 rounded-xl transition"
        :class="
          highlightedId === item.category_id
            ? 'bg-[#F1F3F6]'
            : 'hover:bg-[#F8F9FB]'
        "
        @click="handleSelect(item.category_id)"
      >
        <span class="flex items-center gap-2 min-w-0">
          <span
            class="w-2.5 h-2.5 rounded-full shrink-0"
            :style="{ backgroundColor: item.category_color || '#DE8A1E' }"
          />
          <span class="font-medium text-[#18181B] truncate">
            {{ item.category_name }}
          </span>
        </span>
        <span class="text-[11px] font-semibold text-[#64748B] tabular-nums shrink-0">
          {{ Math.round(item.percentage) }}%
        </span>
      </button>
    </div>
  </div>
</template>
