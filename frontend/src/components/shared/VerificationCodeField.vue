<script setup lang="ts">
import { computed } from 'vue';

export interface CodeDeliveryHint {
  email: string;
  demoCode?: string;
}

const props = withDefaults(
  defineProps<{
    modelValue: string;
    label?: string;
    placeholder?: string;
    required?: boolean;
    cooldown?: number;
    sending?: boolean;
    sentHint?: CodeDeliveryHint | null;
    variant?: 'auth' | 'pc' | 'mobile';
  }>(),
  {
    label: '邮箱验证码',
    placeholder: '请输入 6 位数字验证码',
    required: true,
    cooldown: 0,
    sending: false,
    sentHint: null,
    variant: 'auth',
  }
);

const emit = defineEmits<{
  'update:modelValue': [value: string];
  send: [];
}>();

const hasDigits = computed(() => props.modelValue.trim().length > 0);

const inputClass = computed(() => {
  const base =
    'flex-1 min-w-0 w-full rounded-xl outline-none border transition tabular-nums focus:border-[#DE8A1E] focus:bg-white';
  const heightPad = props.variant === 'mobile' ? 'px-3 py-2.5' : 'h-10 px-3.5';
  const bgBorder =
    props.variant === 'pc'
      ? 'bg-[#F4F5F8] border-[#EAECEF]'
      : 'bg-[#F5F6F8] border-[#E6E9EF]';
  const typo = hasDigits.value
    ? 'text-base md:text-sm font-bold tracking-[0.22em] text-[#18181B]'
    : 'text-base md:text-xs text-[#18181B]';
  return `${base} ${heightPad} ${bgBorder} ${typo}`;
});

const buttonClass = computed(() => {
  const heightPad = props.variant === 'mobile' ? 'px-3.5 py-2.5' : 'h-10 px-3.5';
  return `shrink-0 whitespace-nowrap ${heightPad} rounded-xl bg-[#FFF6E8] border border-[#F2DFBE]/80 text-[#C67612] text-[11px] font-semibold hover:bg-[#FBEBD1] disabled:opacity-60 transition flex items-center gap-1.5`;
});

function handleInput(e: Event) {
  const el = e.target as HTMLInputElement;
  const cleaned = el.value.replace(/\D+/g, '').slice(0, 6);
  if (el.value !== cleaned) {
    el.value = cleaned;
  }
  emit('update:modelValue', cleaned);
}

function fillDemoCode() {
  if (props.sentHint?.demoCode) {
    emit('update:modelValue', props.sentHint.demoCode.slice(0, 6));
  }
}
</script>

<template>
  <div class="space-y-1.5">
    <label
      class="block text-[11px] font-semibold"
      :class="variant === 'mobile' ? 'text-xs font-bold text-[#18181B]' : 'text-[#475569]'"
    >
      {{ label }}
    </label>

    <div class="flex items-center gap-2">
      <input
        :value="modelValue"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        pattern="[0-9]*"
        maxlength="6"
        :required="required"
        :placeholder="placeholder"
        :class="inputClass"
        @input="handleInput"
      />
      <button
        type="button"
        :disabled="cooldown > 0 || sending"
        :class="buttonClass"
        @click="emit('send')"
      >
        <span
          v-if="sending"
          class="w-3 h-3 rounded-full border-2 border-[#DE8A1E]/30 border-t-[#DE8A1E] animate-spin"
        />
        <span>
          {{ sending ? '发送中…' : cooldown > 0 ? `${cooldown}s 后重发` : '获取验证码' }}
        </span>
      </button>
    </div>

    <!-- 发送回执状态条 -->
    <div
      v-if="sentHint"
      class="px-3 py-2 rounded-xl text-[11px] leading-relaxed flex items-center justify-between gap-2"
      :class="
        sentHint.demoCode
          ? 'bg-[#FFF8EE] border border-[#F2DFBE] text-[#8C5510]'
          : 'bg-[#ECF6F0] border border-[#CBE6D6] text-[#2F6E4B]'
      "
    >
      <div class="min-w-0">
        <template v-if="sentHint.demoCode">
          <span>未开启 SMTP 发信，演示验证码：</span>
          <strong class="font-mono tracking-wider text-[#C67612]">{{ sentHint.demoCode }}</strong>
        </template>
        <template v-else>
          <span>验证码已发送至 </span>
          <strong class="font-semibold">{{ sentHint.email }}</strong>
          <span>（10 分钟内有效）</span>
        </template>
      </div>
      <button
        v-if="sentHint.demoCode"
        type="button"
        class="shrink-0 px-2.5 py-1 rounded-lg bg-white border border-[#F2DFBE] text-[10px] font-bold text-[#DE8A1E] hover:bg-[#FFF6E8] transition"
        @click="fillDemoCode"
      >
        一键填入
      </button>
    </div>
  </div>
</template>
