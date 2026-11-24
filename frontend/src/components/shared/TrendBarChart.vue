<script setup lang="ts">
import { computed } from 'vue';
import { formatMoney } from '@/utils/format';

export interface TrendBarItem {
  key: string;
  label: string;
  value: number; // in cents
  rawDate?: string;
}

const props = withDefaults(
  defineProps<{
    items: TrendBarItem[];
    selectedKey?: string | null;
    accentColor?: string;
    mode?: 'mobile' | 'pc';
    privacy?: boolean;
    heightClass?: string;
  }>(),
  {
    selectedKey: null,
    accentColor: '#DE8A1E',
    mode: 'mobile',
    privacy: false,
    heightClass: '',
  }
);

const emit = defineEmits<{
  select: [item: TrendBarItem];
}>();

const maxVal = computed(() => {
  const max = Math.max(0, ...props.items.map((it) => it.value || 0));
  return max > 0 ? max : 10000; // 至少 100 元基准防零除
});

const containerHeightClass = computed(() => {
  if (props.heightClass) return props.heightClass;
  return props.mode === 'pc' ? 'h-44' : 'h-32';
});

function getBarHeightPct(val: number): number {
  if (!val || val <= 0) return 5;
  return Math.max(10, Math.round((val / maxVal.value) * 100));
}

function getMobileBarStyle(item: TrendBarItem) {
  const isSel = props.selectedKey === item.key;
  const backgroundColor = isSel
    ? props.accentColor
    : item.value > 0
      ? '#E2E8F0'
      : '#EEF0F4';
  return {
    height: `${getBarHeightPct(item.value)}%`,
    backgroundColor,
  };
}

function getPcBarStyle(item: TrendBarItem) {
  const isSel = props.selectedKey === item.key;
  const backgroundColor = isSel
    ? props.accentColor
    : item.value > 0
      ? '#FBEBD1'
      : '#F1F3F6';
  return {
    height: `${getBarHeightPct(item.value)}%`,
    backgroundColor,
  };
}
</script>

<template>
  <div
    class="flex items-end justify-between pt-4"
    :class="[
      containerHeightClass,
      mode === 'pc' ? 'gap-2' : 'gap-1.5 px-1',
    ]"
  >
    <button
      v-for="item in items"
      :key="item.key"
      type="button"
      class="flex-1 h-full flex flex-col items-center justify-end gap-1.5 group focus:outline-none"
      @click="emit('select', item)"
    >
      <span
        v-if="mode === 'pc'"
        class="text-[10px] tabular-nums transition-colors leading-none"
        :class="
          selectedKey === item.key
            ? 'text-[#18181B] font-semibold'
            : 'text-[#94A3B8] group-hover:text-[#18181B]'
        "
      >
        {{
          item.value > 0
            ? formatMoney(item.value, { privacy, compact: true, decimals: 0 })
            : ''
        }}
      </span>

      <div
        v-if="mode === 'pc'"
        class="w-full max-w-[30px] rounded-t-lg bar-fill group-hover:brightness-95"
        :style="getPcBarStyle(item)"
      />
      <div
        v-else
        class="w-full max-w-[16px] rounded-t-md bar-fill"
        :style="getMobileBarStyle(item)"
      />

      <span
        class="tabular-nums leading-none"
        :class="[
          mode === 'pc' ? 'text-[10px]' : 'text-[9px]',
          selectedKey === item.key
            ? 'font-bold text-[#18181B]'
            : mode === 'pc'
              ? 'text-[#64748B]'
              : 'text-[#94A3B8]',
        ]"
      >
        {{ item.label }}
      </span>
    </button>
  </div>
</template>
