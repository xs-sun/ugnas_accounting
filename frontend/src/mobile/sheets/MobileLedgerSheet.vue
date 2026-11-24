<script setup lang="ts">
import { ref } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney } from '@/utils/format';
import { resolveAvatarUrl } from '@/constants/avatars';
import type { Book, BookMember } from '@/types';
import BottomSheet from '../components/BottomSheet.vue';

defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
  'open-new-ledger': [];
}>();

const store = useAppStore();
const switchingId = ref('');
const inviteCode = ref('');
const joining = ref(false);

function closeSheet() {
  emit('update:open', false);
  emit('close');
}

async function handleSwitchBook(book: Book) {
  if (switchingId.value) return;
  if (book.id === store.activeBookId) {
    closeSheet();
    return;
  }
  switchingId.value = book.id;
  try {
    await store.switchBook(book.id);
    store.showToast(`已切换至「${book.name}」`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '切换账本失败', 'error');
  } finally {
    switchingId.value = '';
  }
}

function handleOpenNewLedger() {
  closeSheet();
  emit('open-new-ledger');
}

async function handleJoinByCode() {
  const code = inviteCode.value.trim();
  if (!code) {
    store.showToast('请输入账本邀请口令', 'error');
    return;
  }
  if (joining.value) return;
  joining.value = true;
  try {
    const joined = await api.joinBookByCode({ invite_code: code });
    await store.refreshBooks();
    await store.switchBook(joined.id);
    inviteCode.value = '';
    store.showToast(`已成功加入「${joined.name}」`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '加入账本失败，请核对邀请口令', 'error');
  } finally {
    joining.value = false;
  }
}

function hasMemberAvatar(member: BookMember): boolean {
  return Boolean(member.is_me || (member.avatar && member.avatar.trim()));
}

function getMemberAvatar(member: BookMember): string {
  if (member.is_me) return resolveAvatarUrl(store.user?.avatar || member.avatar);
  return resolveAvatarUrl(member.avatar);
}

function getMemberInitial(member: BookMember): string {
  return (member.user_name || '成').trim().slice(0, 1).toUpperCase();
}
</script>

<template>
  <BottomSheet
    :open="open"
    :multi-detent="false"
    max-height-class="max-h-[84dvh]"
    bg-class="bg-[#F8F9FB]"
    z-index-class="z-40"
    @close="emit('close')"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="px-5 py-2 flex items-center justify-between gap-2 shrink-0">
        <h3 class="text-[15px] font-bold text-[#18181B]">账本管理与共享</h3>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="text-[11px] font-semibold px-3 py-1.5 rounded-full bg-[#DE8A1E] text-white hover:bg-[#C67612] transition"
            @click="handleOpenNewLedger"
          >
            + 新建账本
          </button>
          <button
            type="button"
            aria-label="关闭账本抽屉"
            class="w-8 h-8 rounded-full bg-white border border-[#E6E9EF] flex items-center justify-center text-[#64748B] hover:text-[#18181B] transition"
            @click="closeSheet"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </template>

    <!-- 账本列表 -->
    <div class="flex-1 min-h-0 overflow-y-auto phone-scroll px-4 py-2 space-y-2.5">
      <div v-if="store.books.length === 0" class="app-card p-6 text-center text-xs text-[#94A3B8]">
        暂无账本，点击右上角「+ 新建账本」创建一个吧
      </div>

      <button
        v-for="book in store.books"
        :key="book.id"
        type="button"
        :disabled="Boolean(switchingId)"
        class="w-full app-card p-3.5 flex items-center justify-between gap-3 text-left transition"
        :class="
          book.id === store.activeBookId
            ? 'ring-2 ring-[#DE8A1E] bg-[#FFFDF9]'
            : 'hover:bg-[#F8F9FB]'
        "
        @click="handleSwitchBook(book)"
      >
        <div class="flex items-center gap-3 min-w-0">
          <div
            class="w-10 h-10 rounded-2xl flex items-center justify-center text-white font-bold text-sm shrink-0 shadow-xs"
            :style="{ backgroundColor: book.color || '#DE8A1E' }"
          >
            {{ book.name.slice(0, 1) }}
          </div>

          <div class="min-w-0">
            <div class="flex items-center gap-1.5 flex-wrap">
              <p class="text-sm font-bold text-[#18181B] truncate">{{ book.name }}</p>
              <span
                v-if="book.id === store.activeBookId"
                class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-[#FFF6E8] text-[#DE8A1E] shrink-0"
              >
                当前
              </span>
            </div>

            <p class="text-[11px] text-[#64748B] truncate mt-0.5 tabular-nums">
              {{ book.description || '独立核算账本' }} · 预算
              {{
                book.budget > 0
                  ? formatMoney(book.budget, { privacy: store.privacyHidden, decimals: 0 })
                  : '未设'
              }}
              · {{ book.tx_count ?? 0 }} 笔
            </p>

            <div class="flex items-center gap-1.5 mt-1.5">
              <div class="flex -space-x-1">
                <template v-for="m in (book.members || []).slice(0, 5)" :key="m.user_id">
                  <img
                    v-if="hasMemberAvatar(m)"
                    :src="getMemberAvatar(m)"
                    :alt="m.user_name"
                    class="w-5 h-5 rounded-full object-cover ring-1 ring-white bg-white"
                  />
                  <span
                    v-else
                    class="w-5 h-5 rounded-full text-[9px] font-bold text-white flex items-center justify-center ring-1 ring-white"
                    :style="{ backgroundColor: m.color || '#DE8A1E' }"
                  >
                    {{ getMemberInitial(m) }}
                  </span>
                </template>
              </div>
              <span class="text-[10px] text-[#94A3B8]">
                {{ (book.members || []).length || 1 }} 人参与
              </span>
            </div>
          </div>
        </div>

        <svg
          v-if="switchingId === book.id"
          class="w-4 h-4 text-[#DE8A1E] animate-spin shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.2"
        >
          <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4" />
        </svg>
        <svg
          v-else
          class="w-4 h-4 text-[#94A3B8] shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.2"
        >
          <path d="M9 18l6-6-6-6" />
        </svg>
      </button>
    </div>

    <!-- 底部通过口令加入共享账本 -->
    <template #footer>
      <div class="p-4 pt-3 pb-6 border-t border-[#ECEEF2] bg-[#F5F6F8] shrink-0">
        <p class="text-[11px] font-medium text-[#64748B] mb-2">
          通过邀请口令加入家人 / 伙伴的共享账本
        </p>
        <form class="flex items-center gap-2" @submit.prevent="handleJoinByCode">
          <input
            v-model="inviteCode"
            type="text"
            maxlength="24"
            placeholder="输入邀请口令，如 CAT7-8492"
            class="flex-1 min-w-0 text-xs bg-white border border-[#DFE3EA] rounded-xl px-3 py-2.5 outline-none focus:border-[#DE8A1E] transition"
          />
          <button
            type="submit"
            :disabled="joining"
            class="px-3.5 py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition shrink-0"
          >
            {{ joining ? '加入中…' : '加入账本' }}
          </button>
        </form>
      </div>
    </template>
  </BottomSheet>
</template>
