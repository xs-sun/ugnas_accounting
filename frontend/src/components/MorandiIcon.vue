<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(
  defineProps<{
    icon: string;
    size?: number; // default 36px
    color?: string; // custom override
  }>(),
  {
    size: 36,
  }
);

// Morandi color palettes: low saturation background + matching stroke
const palette = computed(() => {
  const k = props.icon.toLowerCase();
  if (k.includes('food') || k.includes('餐') || k.includes('饮')) {
    return { bg: '#F9EFE9', stroke: '#9E6748', type: 'food' };
  }
  if (k.includes('shop') || k.includes('购') || k.includes('买') || k.includes('生活')) {
    return { bg: '#EAF0F6', stroke: '#4C6E8D', type: 'shop' };
  }
  if (k.includes('trans') || k.includes('行') || k.includes('车') || k.includes('路')) {
    return { bg: '#E8F4F2', stroke: '#2E7E73', type: 'trans' };
  }
  if (k.includes('home') || k.includes('房') || k.includes('电') || k.includes('居')) {
    return { bg: '#F2EFE9', stroke: '#7D6F56', type: 'home' };
  }
  if (k.includes('fun') || k.includes('娱') || k.includes('玩') || k.includes('游')) {
    return { bg: '#F4EEF7', stroke: '#7E5B8E', type: 'fun' };
  }
  if (k.includes('health') || k.includes('医') || k.includes('药')) {
    return { bg: '#FAECEB', stroke: '#A85854', type: 'health' };
  }
  if (k.includes('salary') || k.includes('薪') || k.includes('工')) {
    return { bg: '#E4F3F1', stroke: '#0E8C7F', type: 'salary' };
  }
  if (k.includes('invest') || k.includes('理') || k.includes('财')) {
    return { bg: '#FEF4E8', stroke: '#B87528', type: 'invest' };
  }
  if (k.includes('wechat') || k.includes('微信')) {
    return { bg: '#EAF5EE', stroke: '#2E8B57', type: 'wechat' };
  }
  if (k.includes('alipay') || k.includes('支')) {
    return { bg: '#E8F2FA', stroke: '#2577BA', type: 'alipay' };
  }
  if (k.includes('credit') || k.includes('信')) {
    return { bg: '#F7EDEE', stroke: '#A44C57', type: 'credit' };
  }
  if (k.includes('debit') || k.includes('卡') || k.includes('银')) {
    return { bg: '#EEF1F7', stroke: '#415B80', type: 'debit' };
  }
  if (k.includes('cash') || k.includes('现')) {
    return { bg: '#F6F3EC', stroke: '#857351', type: 'cash' };
  }
  return { bg: '#F0F1F3', stroke: '#666F7D', type: 'tag' };
});
</script>

<template>
  <div
    class="flex items-center justify-center shrink-0 select-none transition-transform duration-200"
    :style="{
      width: `${size}px`,
      height: `${size}px`,
      borderRadius: '11px',
      backgroundColor: palette.bg,
      color: color || palette.stroke,
    }"
  >
    <!-- Food Icon -->
    <svg
      v-if="palette.type === 'food'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M18 8h1a4 4 0 0 1 0 8h-1" />
      <path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z" />
      <line x1="6" y1="1" x2="6" y2="4" />
      <line x1="10" y1="1" x2="10" y2="4" />
      <line x1="14" y1="1" x2="14" y2="4" />
    </svg>

    <!-- Shopping Icon -->
    <svg
      v-else-if="palette.type === 'shop'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M6 2L3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" />
      <line x1="3" y1="6" x2="21" y2="6" />
      <path d="M16 10a4 4 0 0 1-8 0" />
    </svg>

    <!-- Transport Icon -->
    <svg
      v-else-if="palette.type === 'trans'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="3" y="3" width="18" height="13" rx="2" />
      <path d="M7 16v3" />
      <path d="M17 16v3" />
      <circle cx="7.5" cy="11.5" r="1.5" fill="currentColor" />
      <circle cx="16.5" cy="11.5" r="1.5" fill="currentColor" />
    </svg>

    <!-- Housing Icon -->
    <svg
      v-else-if="palette.type === 'home'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>

    <!-- Entertainment / Fun Icon -->
    <svg
      v-else-if="palette.type === 'fun'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M8 14s1.5 2 4 2 4-2 4-2" />
      <line x1="9" y1="9" x2="9.01" y2="9" />
      <line x1="15" y1="9" x2="15.01" y2="9" />
    </svg>

    <!-- Health Icon -->
    <svg
      v-else-if="palette.type === 'health'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="6" width="20" height="14" rx="2" />
      <path d="M12 10v6" />
      <path d="M9 13h6" />
      <path d="M16 6V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2" />
    </svg>

    <!-- Salary Icon -->
    <svg
      v-else-if="palette.type === 'salary'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="4" width="20" height="16" rx="2" />
      <line x1="2" y1="10" x2="22" y2="10" />
      <circle cx="12" cy="15" r="2" />
    </svg>

    <!-- Investment Icon -->
    <svg
      v-else-if="palette.type === 'invest'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
      <polyline points="16 7 22 7 22 13" />
    </svg>

    <!-- WeChat Icon -->
    <svg
      v-else-if="palette.type === 'wechat'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M9 11a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM15 11a1 1 0 1 0 0-2 1 1 0 0 0 0 2z" />
      <path d="M18.5 12c.3 0 .6 0 .9.1A6 6 0 0 0 8.2 6.5 7 7 0 0 0 1 13.5c0 2 1 3.8 2.5 5L3 21l3-1c1 .4 2 .5 3 .5a7 7 0 0 0 6.6-4.5 5 5 0 0 1 2.9-4z" />
    </svg>

    <!-- Alipay Icon -->
    <svg
      v-else-if="palette.type === 'alipay'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="3" y="3" width="18" height="18" rx="4" />
      <path d="M7 9h10" />
      <path d="M12 9v4c0 2-1 3-3 3" />
      <path d="M10 13c2 1 4 2 7 3" />
    </svg>

    <!-- Credit Card Icon -->
    <svg
      v-else-if="palette.type === 'credit'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="5" width="20" height="14" rx="2" />
      <line x1="2" y1="10" x2="22" y2="10" />
      <circle cx="7" cy="15" r="1" fill="currentColor" />
    </svg>

    <!-- Bank / Debit Card Icon -->
    <svg
      v-else-if="palette.type === 'debit'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="5" width="20" height="14" rx="2" />
      <line x1="2" y1="10" x2="22" y2="10" />
      <line x1="6" y1="15" x2="10" y2="15" />
    </svg>

    <!-- Cash Icon -->
    <svg
      v-else-if="palette.type === 'cash'"
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="6" width="20" height="12" rx="2" />
      <circle cx="12" cy="12" r="2" />
      <path d="M6 12h.01M18 12h.01" />
    </svg>

    <!-- Fallback Tag Icon -->
    <svg
      v-else
      class="w-1/2 h-1/2"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z" />
      <line x1="7" y1="7" x2="7.01" y2="7" />
    </svg>
  </div>
</template>
