<script setup lang="ts">
import { computed } from 'vue';
import { formatMoney } from '@/utils/format';

const props = withDefaults(
  defineProps<{
    percentage: number;
    spent: number; // in cents
    limit: number; // in cents
    privacy?: boolean;
    showSummary?: boolean;
    heightClass?: string;
    accentColor?: string;
  }>(),
  {
    privacy: false,
    showSummary: false,
    heightClass: 'h-2',
    accentColor: '',
  }
);

const clampedPct = computed(() =>
  Math.min(100, Math.max(0, Math.round(props.percentage || 0)))
);

const isOver = computed(() => props.limit > 0 && props.spent > props.limit);

const barColor = computed(() => {
  if (isOver.value || props.percentage >= 85) return '#D6553E';
  if (props.percentage >= 65) return '#D9822B';
  return props.accentColor || '#DE8A1E';
});

const remaining = computed(() => Math.max(0, props.limit - props.spent));
</script>

<template>
  <div>
    <div class="rounded-full bg-[#E5E8EE] overflow-hidden" :class="heightClass">
      <div
        class="h-full rounded-full transition-all duration-300"
        :style="{ width: `${clampedPct}%`, backgroundColor: barColor }"
      />
    </div>
    <div
      v-if="showSummary"
      class="flex items-center justify-between text-[11px] text-[#64748B] tabular-nums mt-1.5"
    >
      <span>
        已用 {{ formatMoney(spent, { privacy }) }}
        <span class="font-semibold" :style="{ color: barColor }">
          ({{ Math.round(percentage || 0) }}%)
        </span>
      </span>
      <span v-if="isOver" class="font-semibold text-[#D6553E]">
        已超支 {{ formatMoney(spent - limit, { privacy }) }}
      </span>
      <span v-else>
        剩余 {{ formatMoney(remaining, { privacy }) }}
      </span>
    </div>
  </div>
</template>
