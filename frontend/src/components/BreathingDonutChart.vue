<script setup lang="ts">
import { ref, computed } from 'vue';
import type { CategoryReportItem } from '../types';

const props = defineProps<{
  categories: CategoryReportItem[];
  totalExpense: number; // in cents
}>();

const hoveredIndex = ref<number | null>(null);

// Morandi pastel color sequence
const morandiColors = [
  '#E5654E', // Terracotta Orange
  '#2E7E73', // Pine Turquoise
  '#4C6E8D', // Slate Blue
  '#B87528', // Warm Amber
  '#7E5B8E', // Heather Purple
  '#447C5E', // Forest Sage
  '#A85854', // Muted Coral
  '#945369', // Mauve
  '#7D6F56', // Olive Drab
  '#506385', // Steel Blue
];

interface DonutSlice {
  category: CategoryReportItem;
  color: string;
  startAngle: number;
  endAngle: number;
  path: string;
  percentage: number;
}

const size = 220;
const center = size / 2;
const radius = 88;
const innerRadius = 66;

function polarToCartesian(centerX: number, centerY: number, r: number, angleInDegrees: number) {
  const angleInRadians = ((angleInDegrees - 90) * Math.PI) / 180.0;
  return {
    x: centerX + r * Math.cos(angleInRadians),
    y: centerY + r * Math.sin(angleInRadians),
  };
}

function describeArc(
  x: number,
  y: number,
  rOut: number,
  rIn: number,
  startAngle: number,
  endAngle: number
) {
  // Clamp angle delta so it doesn't wrap 360 awkwardly
  let delta = endAngle - startAngle;
  if (delta >= 359.99) delta = 359.99;
  const safeEnd = startAngle + delta;

  const startOut = polarToCartesian(x, y, rOut, startAngle);
  const endOut = polarToCartesian(x, y, rOut, safeEnd);
  const startIn = polarToCartesian(x, y, rIn, safeEnd);
  const endIn = polarToCartesian(x, y, rIn, startAngle);

  const largeArcFlag = delta <= 180 ? '0' : '1';

  return [
    'M', startOut.x, startOut.y,
    'A', rOut, rOut, 0, largeArcFlag, 1, endOut.x, endOut.y,
    'L', startIn.x, startIn.y,
    'A', rIn, rIn, 0, largeArcFlag, 0, endIn.x, endIn.y,
    'Z',
  ].join(' ');
}

const slices = computed<DonutSlice[]>(() => {
  if (!props.categories || props.categories.length === 0 || props.totalExpense === 0) {
    return [];
  }

  let currentAngle = 0;
  return props.categories.map((cat, idx) => {
    const pct = cat.amount / props.totalExpense;
    const angleDelta = pct * 360;
    const startAngle = currentAngle;
    const endAngle = currentAngle + angleDelta;
    currentAngle = endAngle;

    const color = morandiColors[idx % morandiColors.length];
    const path = describeArc(center, center, radius, innerRadius, startAngle, endAngle);

    return {
      category: cat,
      color,
      startAngle,
      endAngle,
      path,
      percentage: Math.round(pct * 1000) / 10,
    };
  });
});

const activeLabel = computed(() => {
  if (hoveredIndex.value !== null && slices.value[hoveredIndex.value]) {
    const s = slices.value[hoveredIndex.value];
    return {
      title: s.category.category_name,
      amount: (s.category.amount / 100).toFixed(2),
      sub: `${s.percentage}%`,
    };
  }
  return {
    title: '本月支出',
    amount: (props.totalExpense / 100).toFixed(2),
    sub: '总计',
  };
});
</script>

<template>
  <div class="relative flex items-center justify-center select-none">
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" class="overflow-visible">
      <!-- Background Empty Track -->
      <circle
        v-if="slices.length === 0"
        :cx="center"
        :cy="center"
        :r="(radius + innerRadius) / 2"
        fill="none"
        stroke="#EDEEF1"
        :stroke-width="radius - innerRadius"
      />

      <!-- Donut Slices with White Breathing Gap -->
      <g v-else>
        <path
          v-for="(slice, i) in slices"
          :key="i"
          :d="slice.path"
          :fill="slice.color"
          stroke="#FFFFFF"
          stroke-width="2.5"
          stroke-linejoin="round"
          class="cursor-pointer transition-all duration-200"
          :class="{
            'opacity-100 scale-[1.02] filter drop-shadow-sm': hoveredIndex === i,
            'opacity-90 hover:opacity-100': hoveredIndex === null,
            'opacity-40': hoveredIndex !== null && hoveredIndex !== i,
          }"
          :style="{ transformOrigin: `${center}px ${center}px` }"
          @mouseenter="hoveredIndex = i"
          @mouseleave="hoveredIndex = null"
        />
      </g>
    </svg>

    <!-- Center Floating Label -->
    <div
      class="absolute flex flex-col items-center justify-center text-center pointer-events-none transition-all duration-200"
      :style="{ width: `${innerRadius * 1.8}px` }"
    >
      <span class="text-xs text-[#5D6470] font-medium leading-none mb-1 truncate w-full">
        {{ activeLabel.title }}
      </span>
      <span class="text-lg font-bold text-[#171A20] tabular-nums tracking-tightest leading-none">
        ¥{{ activeLabel.amount }}
      </span>
      <span class="text-[11px] text-[#9AA0AB] mt-1 font-medium">
        {{ activeLabel.sub }}
      </span>
    </div>
  </div>
</template>
