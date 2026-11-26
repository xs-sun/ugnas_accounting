<script setup lang="ts">
import { computed } from 'vue';
import { evalSimpleExpr } from '@/utils/format';

const props = withDefaults(
  defineProps<{
    modelValue: string;
    type?: 'expense' | 'income';
    currencySymbol?: string;
    submitting?: boolean;
    showHeader?: boolean;
  }>(),
  {
    type: 'expense',
    currencySymbol: '¥',
    submitting: false,
    showHeader: true,
  }
);

const emit = defineEmits<{
  'update:modelValue': [expr: string];
  submit: [keepOpen: boolean];
}>();

const evaluatedAmount = computed(() => evalSimpleExpr(props.modelValue));

const hasOperator = computed(() => /[+\-]/.test(props.modelValue));

const displayAmountText = computed(() => {
  if (!props.modelValue) return '0.00';
  return props.modelValue;
});

const signColorClass = computed(() =>
  props.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'
);

function getCurrentNumberSegment(expr: string): string {
  const parts = expr.split(/[+\-]/);
  return parts[parts.length - 1] || '';
}

function pressKey(key: string) {
  if (props.submitting) return;
  // 按键轻震一下，模拟系统键盘的手感；不支持震动的设备（iOS WebView
  // 没有 Vibration API）直接忽略，不报错也不影响输入。
  try {
    if (typeof navigator !== 'undefined' && 'vibrate' in navigator) {
      navigator.vibrate(10);
    }
  } catch {
    /* ignore */
  }
  const cur = props.modelValue || '';

  if (key === 'BACK') {
    emit('update:modelValue', cur.slice(0, -1));
    return;
  }
  if (key === 'AGAIN') {
    emit('submit', true);
    return;
  }
  if (key === 'DONE') {
    emit('submit', false);
    return;
  }

  if (cur.length >= 18) return;

  if (key === '+' || key === '-') {
    if (!cur) return;
    if (/[+\-]$/.test(cur)) {
      emit('update:modelValue', cur.slice(0, -1) + key);
    } else {
      emit('update:modelValue', cur + key);
    }
    return;
  }

  const seg = getCurrentNumberSegment(cur);

  if (key === '.') {
    if (seg.includes('.')) return;
    if (!seg) {
      emit('update:modelValue', cur + '0.');
    } else {
      emit('update:modelValue', cur + '.');
    }
    return;
  }

  // 限制当前数字段小数点后最多两位
  if (seg.includes('.')) {
    const decimals = seg.split('.')[1] || '';
    if (decimals.length >= 2) return;
  }

  // 避免前导多余 0（如 00 -> 0）
  if (seg === '0' && key === '0') return;
  if (seg === '0' && key !== '0') {
    emit('update:modelValue', cur.slice(0, -1) + key);
    return;
  }

  emit('update:modelValue', cur + key);
}
</script>

<template>
  <div
    class="shrink-0 bg-white border-t border-[#ECEEF2] rounded-t-[24px] px-4 pt-3 pb-4 shadow-[0_-6px_20px_rgba(15,23,42,0.04)]"
  >
    <!-- 金额大字展示 + 算式预览 + 右侧备注插槽 -->
    <div
      v-if="showHeader"
      class="flex items-center justify-between gap-3 pb-2.5 border-b border-[#F1F3F6]"
    >
      <div class="flex items-baseline gap-1 min-w-0 shrink-0 max-w-[60%]">
        <span class="text-base font-bold shrink-0" :class="signColorClass">
          {{ currencySymbol }}
        </span>
        <span
          class="text-[26px] leading-none font-bold tracking-tight tabular-nums text-[#18181B] truncate"
        >
          {{ displayAmountText }}
        </span>
        <span
          v-if="hasOperator"
          class="ml-1 text-[11px] font-semibold text-[#DE8A1E] tabular-nums shrink-0"
        >
          = {{ currencySymbol }}{{ evaluatedAmount.toFixed(2) }}
        </span>
      </div>
      <div class="flex-1 min-w-0 flex justify-end">
        <slot name="header-right" />
      </div>
    </div>

    <!-- 日期/账户/开关等元数据插槽 -->
    <slot name="meta" />

    <!-- 4x4 计算器小键盘：7-9/⌫, 4-6/+, 1-3/-, 再记/0/./完成 -->
    <div class="keypad-keys grid grid-cols-4 gap-1.5 pt-0.5 select-none">
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('7')"
      >
        7
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('8')"
      >
        8
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('9')"
      >
        9
      </button>
      <button
        type="button"
        aria-label="退格"
        class="key-btn h-10 rounded-xl bg-[#ECEEF2] text-sm font-semibold text-[#475569] flex items-center justify-center transition"
        @click="pressKey('BACK')"
      >
        ⌫
      </button>

      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('4')"
      >
        4
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('5')"
      >
        5
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('6')"
      >
        6
      </button>
      <button
        type="button"
        aria-label="加"
        class="key-btn h-10 rounded-xl bg-[#ECEEF2] text-base font-semibold text-[#475569] transition"
        @click="pressKey('+')"
      >
        +
      </button>

      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('1')"
      >
        1
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('2')"
      >
        2
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('3')"
      >
        3
      </button>
      <button
        type="button"
        aria-label="减"
        class="key-btn h-10 rounded-xl bg-[#ECEEF2] text-base font-semibold text-[#475569] transition"
        @click="pressKey('-')"
      >
        −
      </button>

      <button
        type="button"
        :disabled="submitting"
        class="key-btn h-10 rounded-xl bg-[#FFF6E8] text-xs font-semibold text-[#DE8A1E] disabled:opacity-50 transition"
        @click="pressKey('AGAIN')"
      >
        再记
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-[15px] font-semibold text-[#18181B] transition"
        @click="pressKey('0')"
      >
        0
      </button>
      <button
        type="button"
        class="key-btn h-10 rounded-xl bg-[#F5F6F8] text-base font-bold text-[#18181B] transition"
        @click="pressKey('.')"
      >
        .
      </button>
      <button
        type="button"
        :disabled="submitting"
        class="key-btn h-10 rounded-xl bg-[#DE8A1E] text-xs font-semibold text-white shadow-xs disabled:opacity-60 transition"
        @click="pressKey('DONE')"
      >
        {{ submitting ? '保存中…' : '完成' }}
      </button>
    </div>
  </div>
</template>
