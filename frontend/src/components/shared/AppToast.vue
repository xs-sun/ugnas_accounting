<script setup lang="ts">
import { computed } from 'vue';
import { useAppStore } from '@/stores/useAppStore';

const props = withDefaults(
  defineProps<{
    topClass?: string;
  }>(),
  {
    topClass: 'top-5',
  }
);

const store = useAppStore();

const dotColorClass = computed(() => {
  if (store.toast.type === 'success') return 'bg-[#3E8E63]';
  if (store.toast.type === 'error') return 'bg-[#D6553E]';
  return 'bg-[#F4B65B]';
});
</script>

<template>
  <div
    class="toast-pill fixed left-1/2 -translate-x-1/2 z-[90] bg-[#18181B] text-white text-xs font-medium px-4 py-2.5 rounded-full shadow-xl flex items-center gap-2 whitespace-nowrap"
    :class="[props.topClass, { 'toast-hidden': !store.toast.visible }]"
    role="status"
    aria-live="polite"
  >
    <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="dotColorClass" />
    <span>{{ store.toast.message }}</span>
  </div>
</template>
