<script setup lang="ts">
import type { BookMember } from '@/types';
import { resolveAvatarUrl } from '@/constants/avatars';

defineProps<{
  open: boolean;
  members: BookMember[];
  modelValue: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [userId: string];
  'update:open': [open: boolean];
  close: [];
}>();

function close() {
  emit('update:open', false);
  emit('close');
}

function select(m: BookMember) {
  emit('update:modelValue', m.user_id);
  close();
}

function memberInitial(m: BookMember): string {
  return (m.user_name || '?').slice(0, 1).toUpperCase();
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[100] flex items-end justify-center">
      <div class="absolute inset-0 bg-[#14171A]/50" @click="close" />
      <div
        class="relative w-full max-w-md bg-white rounded-t-3xl px-4 pt-2 pb-6 max-h-[70vh] overflow-y-auto phone-scroll"
      >
        <div class="w-10 h-1 rounded-full bg-[#E6E9EF] mx-auto my-2" />
        <p class="text-sm font-bold text-[#18181B] text-center mb-3">选择成员</p>
        <p class="text-[11px] text-[#94A3B8] text-center -mt-2 mb-3">
          这笔流水算在谁头上（可与记账人不同）
        </p>
        <div v-if="members.length === 0" class="py-6 text-center text-xs text-[#94A3B8]">
          当前账本暂无成员
        </div>
        <button
          v-for="m in members"
          :key="m.user_id"
          type="button"
          class="w-full flex items-center justify-between px-3 py-2.5 rounded-xl text-sm transition"
          :class="
            modelValue === m.user_id
              ? 'bg-[#FFF6E8] text-[#18181B] font-semibold'
              : 'text-[#475569] hover:bg-[#F8F9FB]'
          "
          @click="select(m)"
        >
          <span class="flex items-center gap-2.5 min-w-0">
            <img
              v-if="m.avatar"
              :src="resolveAvatarUrl(m.avatar)"
              :alt="m.user_name"
              class="w-8 h-8 rounded-full object-cover bg-white border border-[#ECEEF2] shrink-0"
            />
            <span
              v-else
              class="w-8 h-8 rounded-full text-xs font-bold text-white flex items-center justify-center shrink-0"
              :style="{ backgroundColor: m.color || '#DE8A1E' }"
            >
              {{ memberInitial(m) }}
            </span>
            <span class="flex items-center gap-1.5 min-w-0">
              <span class="truncate">{{ m.user_name }}</span>
              <span
                v-if="m.is_me"
                class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-[#F1F3F6] text-[#475569] shrink-0"
              >
                我
              </span>
              <span
                v-if="m.is_virtual"
                class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-[#EEF2FF] text-[#4F46E5] shrink-0"
              >
                虚拟成员
              </span>
            </span>
          </span>
          <span v-if="modelValue === m.user_id" class="text-[#DE8A1E] font-bold shrink-0">✓</span>
        </button>
      </div>
    </div>
  </Teleport>
</template>
