<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    disabled?: boolean;
    ariaLabel?: string;
  }>(),
  {
    disabled: false,
    ariaLabel: '切换开关',
  }
);

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  change: [value: boolean];
}>();

function toggle() {
  if (props.disabled) return;
  const next = !props.modelValue;
  emit('update:modelValue', next);
  emit('change', next);
}
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="ariaLabel"
    :disabled="disabled"
    class="w-11 h-6 rounded-full p-0.5 transition shrink-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#DE8A1E]"
    :class="[
      modelValue ? 'bg-[#DE8A1E]' : 'bg-[#D4D9E1]',
      disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
    ]"
    @click="toggle"
  >
    <span
      class="block w-5 h-5 rounded-full bg-white shadow-sm transition-transform"
      :class="modelValue ? 'translate-x-5' : 'translate-x-0'"
    />
  </button>
</template>
