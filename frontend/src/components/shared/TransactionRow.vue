<script setup lang="ts">
import { computed } from 'vue';
import type { Transaction } from '@/types';
import { useAppStore } from '@/stores/useAppStore';
import { resolveAvatarUrl } from '@/constants/avatars';
import { formatMoney } from '@/utils/format';
import CategoryBadge from './CategoryBadge.vue';

const props = withDefaults(
  defineProps<{
    tx: Transaction;
    privacy?: boolean;
    showReimburse?: boolean;
    dense?: boolean;
  }>(),
  {
    privacy: false,
    showReimburse: true,
    dense: false,
  }
);

const emit = defineEmits<{
  click: [tx: Transaction];
}>();

const store = useAppStore();

const memberInfo = computed(() => {
  const members = store.activeBook?.members || [];
  // 归属成员优先：member_id 为空（历史流水）时兜底为创建人。
  const memberId = props.tx.member_id || props.tx.created_by;
  const matched = members.find((m) => m.user_id === memberId);
  const name =
    props.tx.member_name || props.tx.created_by_name || matched?.user_name || '';
  const avatar = props.tx.member_avatar || props.tx.created_by_avatar || matched?.avatar || '';
  const color = props.tx.member_color || matched?.color || '#DE8A1E';
  return { name, avatar, color };
});

const displayTitle = computed(
  () => props.tx.title || props.tx.category_name || props.tx.note || '未命名账单'
);

const subtitleText = computed(() => {
  const parts = [
    props.tx.sub_category,
    props.tx.tx_time || props.tx.date,
    props.tx.wallet_name,
  ].filter(Boolean);
  return parts.join(' · ');
});

const amountColorClass = computed(() => {
  if (props.tx.type === 'income') return 'text-[#3E8E63]';
  if (props.tx.type === 'transfer') return 'text-[#475569]';
  return 'text-[#D6553E]';
});

const formattedAmount = computed(() => {
  if (props.tx.type === 'transfer') {
    return formatMoney(props.tx.amount, { privacy: props.privacy });
  }
  return formatMoney(props.tx.amount, {
    privacy: props.privacy,
    showSign: true,
    type: props.tx.type,
  });
});
</script>

<template>
  <button
    type="button"
    class="w-full px-4 flex items-center justify-between gap-3 hover:bg-[#F8F9FB] transition text-left"
    :class="dense ? 'py-2.5' : 'py-3'"
    @click="emit('click', tx)"
  >
    <div class="flex items-center gap-3 min-w-0">
      <CategoryBadge
        :icon="tx.category_icon || (tx.type === 'transfer' ? 'refund' : 'tag')"
        :color="tx.category_color || (tx.type === 'income' ? '#3E8E63' : '#64748B')"
        :bg-color="tx.category_bg_color || (tx.type === 'income' ? '#ECF6F0' : '#F1F5F9')"
        size="sm"
      />
      <div class="min-w-0">
        <div class="flex items-center gap-1.5">
          <p class="text-[13px] font-semibold text-[#18181B] truncate">
            {{ displayTitle }}
          </p>
          <span
            v-if="showReimburse && tx.reimburse"
            class="text-[9px] px-1.5 py-0.5 rounded bg-[#FEF6E7] text-[#D9822B] font-semibold shrink-0"
          >
            待报销
          </span>
        </div>
        <div class="flex items-center gap-1.5 mt-0.5 min-w-0">
          <p class="text-[11px] text-[#94A3B8] truncate">
            {{ subtitleText }}
          </p>
          <span
            v-if="memberInfo.name"
            class="inline-flex items-center gap-1 text-[10px] text-[#64748B] bg-[#F5F6F8] px-1.5 py-0.5 rounded-full shrink-0 max-w-[96px]"
          >
            <img
              v-if="memberInfo.avatar"
              :src="resolveAvatarUrl(memberInfo.avatar)"
              :alt="memberInfo.name"
              class="w-3.5 h-3.5 rounded-full object-cover shrink-0"
            />
            <span
              v-else
              class="w-3.5 h-3.5 rounded-full text-[8px] font-bold text-white flex items-center justify-center shrink-0"
              :style="{ backgroundColor: memberInfo.color }"
            >
              {{ memberInfo.name.slice(0, 1).toUpperCase() }}
            </span>
            <span class="truncate">{{ memberInfo.name }}</span>
          </span>
        </div>
      </div>
    </div>

    <span
      class="text-[14px] font-bold tabular-nums shrink-0"
      :class="amountColorClass"
    >
      {{ formattedAmount }}
    </span>
  </button>
</template>
