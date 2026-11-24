<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';

export interface DropdownOption {
  value: string | number;
  label: string;
  dot?: string;
  disabled?: boolean;
}

const props = withDefaults(
  defineProps<{
    modelValue: string | number;
    options: DropdownOption[];
    placeholder?: string;
    widthClass?: string;
    size?: 'sm' | 'md';
  }>(),
  {
    placeholder: '请选择',
    widthClass: 'w-full',
    size: 'md',
  }
);

const emit = defineEmits<{
  'update:modelValue': [value: string | number];
  change: [value: string | number];
}>();

const open = ref(false);
const rootRef = ref<HTMLElement | null>(null);

const activeOption = computed<DropdownOption | undefined>(() =>
  props.options.find((o) => o.value === props.modelValue)
);

function toggle() {
  open.value = !open.value;
}

function selectOption(opt: DropdownOption) {
  if (opt.disabled) return;
  open.value = false;
  if (opt.value !== props.modelValue) {
    emit('update:modelValue', opt.value);
    emit('change', opt.value);
  }
}

function onDocPointerDown(e: PointerEvent) {
  if (!open.value || !rootRef.value) return;
  const target = e.target as Node | null;
  if (target && !rootRef.value.contains(target)) {
    open.value = false;
  }
}

function onDocKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && open.value) {
    open.value = false;
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocPointerDown);
  document.addEventListener('keydown', onDocKeyDown);
});

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointerDown);
  document.removeEventListener('keydown', onDocKeyDown);
});
</script>

<template>
  <div ref="rootRef" class="relative" :class="widthClass">
    <button
      type="button"
      class="w-full px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#ECEEF3] border flex items-center justify-between gap-2 text-xs font-semibold text-[#18181B] transition"
      :class="[
        size === 'sm' ? 'h-8' : 'h-9',
        open ? 'border-[#DE8A1E] bg-white' : 'border-[#EAECEF]',
      ]"
      @click="toggle"
    >
      <span class="flex items-center gap-2 min-w-0 truncate">
        <span
          v-if="activeOption?.dot"
          class="w-2 h-2 rounded-full shrink-0"
          :style="{ backgroundColor: activeOption.dot }"
        />
        <span class="truncate" :class="{ 'text-[#94A3B8] font-normal': !activeOption }">
          {{ activeOption ? activeOption.label : placeholder }}
        </span>
      </span>
      <svg
        class="w-3.5 h-3.5 text-[#64748B] shrink-0 transition-transform"
        :class="{ 'rotate-180 text-[#DE8A1E]': open }"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.2"
      >
        <path d="M6 9l6 6 6-6" />
      </svg>
    </button>

    <div
      v-if="open"
      class="cselect-menu absolute left-0 right-0 mt-1.5 z-50 bg-white rounded-2xl p-1.5 min-w-[160px] max-h-60 overflow-y-auto thin-scroll"
    >
      <button
        v-for="opt in options"
        :key="String(opt.value)"
        type="button"
        :disabled="opt.disabled"
        class="w-full px-2.5 py-2 rounded-xl flex items-center justify-between gap-2 text-xs transition"
        :class="[
          opt.disabled
            ? 'opacity-45 cursor-not-allowed text-[#94A3B8]'
            : opt.value === modelValue
              ? 'bg-[#FFF6E8] text-[#DE8A1E] font-bold'
              : 'text-[#3F3F46] hover:bg-[#F4F5F8] font-medium',
        ]"
        @click="selectOption(opt)"
      >
        <span class="flex items-center gap-2 truncate">
          <span
            v-if="opt.dot"
            class="w-2 h-2 rounded-full shrink-0"
            :style="{ backgroundColor: opt.dot }"
          />
          <span class="truncate">{{ opt.label }}</span>
        </span>
        <svg
          v-if="opt.value === modelValue"
          class="w-3.5 h-3.5 shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.4"
        >
          <path d="M5 13l4 4L19 7" />
        </svg>
      </button>
    </div>
  </div>
</template>
