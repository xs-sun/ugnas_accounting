<script setup lang="ts">
import { computed } from 'vue';
import { getIconSvg } from '@/constants/icons';

const props = withDefaults(
  defineProps<{
    icon?: string;
    color?: string;
    bgColor?: string;
    size?: 'xs' | 'sm' | 'md' | 'lg';
    selected?: boolean;
  }>(),
  {
    icon: 'tag',
    color: '#64748B',
    bgColor: '#F1F5F9',
    size: 'md',
    selected: false,
  }
);

const boxClass = computed(() => {
  switch (props.size) {
    case 'xs':
      return 'w-6 h-6 rounded-lg';
    case 'sm':
      return 'w-9 h-9 rounded-[13px]';
    case 'lg':
      return 'w-12 h-12 rounded-2xl';
    case 'md':
    default:
      return 'w-10 h-10 rounded-[14px]';
  }
});

const svgClass = computed(() => {
  switch (props.size) {
    case 'xs':
      return 'w-3.5 h-3.5';
    case 'sm':
      return 'w-[18px] h-[18px]';
    case 'lg':
      return 'w-6 h-6';
    case 'md':
    default:
      return 'w-[21px] h-[21px]';
  }
});

const svgMarkup = computed(() => getIconSvg(props.icon));

const badgeStyle = computed(() => ({
  backgroundColor: props.selected ? props.color : props.bgColor,
  color: props.selected ? '#FFFFFF' : props.color,
}));
</script>

<template>
  <span
    class="inline-flex items-center justify-center shrink-0 transition shadow-[inset_0_1px_1px_rgba(255,255,255,0.68)]"
    :class="boxClass"
    :style="badgeStyle"
  >
    <svg
      :class="svgClass"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
      v-html="svgMarkup"
    />
  </span>
</template>
