<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { DEFAULT_MASCOT, MASCOT_FALLBACK, resolveAvatarUrl } from '@/constants/avatars';
import { LEDGER_COLOR_PRESETS } from '@/constants/icons';
import type { BookMember } from '@/types';
import type { InviteSheetView } from '../types';
import BottomSheet from '../components/BottomSheet.vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    initialView?: InviteSheetView;
  }>(),
  {
    initialView: 'members',
  }
);

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
}>();

const store = useAppStore();

const view = ref<InviteSheetView>(props.initialView);
const members = ref<BookMember[]>([]);
const loadingMembers = ref(false);

// 邀请表单
const inviteRole = ref<'editor' | 'viewer'>('editor');
const inviteEmail = ref('');
const inviting = ref(false);

// 虚拟成员表单
const virtualName = ref('');
const virtualColor = ref(LEDGER_COLOR_PRESETS[2] || '#3A7CA5');
const addingVirtual = ref(false);

// 成员移除状态
const removingUserId = ref('');

const headerTitle = computed(() => {
  if (view.value === 'invite') return '邀请成员加入';
  if (view.value === 'virtual') return '添加虚拟成员';
  return `${store.activeBook?.name || '当前账本'} · 成员`;
});

const inviteCodeText = computed(() => store.activeBook?.invite_code || '暂未生成');

function closeSheet() {
  emit('update:open', false);
  emit('close');
}

async function loadMembers() {
  const bid = store.activeBookId;
  if (!bid) {
    members.value = [];
    return;
  }
  loadingMembers.value = true;
  try {
    members.value = await api.listMembers(bid);
  } catch {
    members.value = store.activeBook?.members || [];
  } finally {
    loadingMembers.value = false;
  }
}

watch(
  () => props.open,
  (opened) => {
    if (opened) {
      view.value = props.initialView || 'members';
      inviteEmail.value = '';
      virtualName.value = '';
      void loadMembers();
    }
  },
  { immediate: true }
);

function roleText(member: BookMember): string {
  if (member.status === 'invited') return '待接受';
  if (member.is_virtual) return '分摊成员';
  if (member.role === 'owner') return '管理员';
  if (member.role === 'viewer') return '仅查看';
  return '可记账';
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

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

async function copyInviteCode() {
  const code = store.activeBook?.invite_code || '';
  if (!code) {
    store.showToast('当前账本暂无可用邀请口令', 'error');
    return;
  }
  const text = `邀请你加入七猫记账「${store.activeBook?.name || '共享账本'}」，邀请口令：${code}`;
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
    store.showToast(`已复制邀请口令 ${code}`, 'success');
  } catch {
    store.showToast(`请手动复制邀请口令：${code}`, 'info', 3500);
  }
}

async function handleInviteByEmail() {
  const email = inviteEmail.value.trim();
  if (!email) {
    store.showToast('请输入对方邮箱地址', 'error');
    return;
  }
  if (!store.activeBookId || inviting.value) return;
  inviting.value = true;
  try {
    const res = await api.inviteMemberByEmail(store.activeBookId, {
      email,
      role: inviteRole.value,
    });
    inviteEmail.value = '';
    await Promise.allSettled([loadMembers(), store.refreshBooks()]);
    view.value = 'members';
    store.showToast(
      res.smtp_sent
        ? `邀请邮件已发送至 ${email}`
        : `已生成邀请记录，口令 ${res.invite_code}`,
      'success'
    );
  } catch (err: any) {
    store.showToast(err?.message || '发送邀请失败', 'error');
  } finally {
    inviting.value = false;
  }
}

async function handleCreateVirtualMember() {
  const name = virtualName.value.trim();
  if (!name) {
    store.showToast('请输入虚拟成员昵称', 'error');
    return;
  }
  if (!store.activeBookId || addingVirtual.value) return;
  addingVirtual.value = true;
  try {
    await api.createVirtualMember(store.activeBookId, {
      name,
      color: virtualColor.value,
    });
    virtualName.value = '';
    await Promise.allSettled([loadMembers(), store.refreshBooks()]);
    view.value = 'members';
    store.showToast(`已添加虚拟成员「${name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '添加虚拟成员失败', 'error');
  } finally {
    addingVirtual.value = false;
  }
}

async function handleRemoveMember(member: BookMember) {
  if (member.is_me || !store.activeBookId || removingUserId.value) return;
  removingUserId.value = member.user_id;
  try {
    await api.removeMember(store.activeBookId, member.user_id);
    await Promise.allSettled([loadMembers(), store.refreshBooks()]);
    store.showToast(`已移除成员「${member.user_name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '移除成员失败', 'error');
  } finally {
    removingUserId.value = '';
  }
}
</script>

<template>
  <BottomSheet
    :open="open"
    :multi-detent="false"
    max-height-class="max-h-[88dvh]"
    bg-class="bg-[#F5F6F8]"
    z-index-class="z-40"
    @close="emit('close')"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="px-5 py-2 flex items-center justify-between gap-2 shrink-0">
        <div class="flex items-center gap-2 min-w-0">
          <button
            v-if="view !== 'members'"
            type="button"
            aria-label="返回成员列表"
            class="w-7 h-7 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#475569] hover:text-[#18181B] shrink-0 transition"
            @click="view = 'members'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          </button>
          <h3 class="text-[15px] font-bold text-[#18181B] truncate">{{ headerTitle }}</h3>
        </div>

        <button
          type="button"
          aria-label="关闭成员抽屉"
          class="w-8 h-8 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#64748B] hover:text-[#18181B] shrink-0 transition"
          @click="closeSheet"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    </template>

    <div class="flex-1 min-h-0 overflow-y-auto phone-scroll px-4 pt-1 pb-6">
      <!-- 态 1：成员列表 -->
      <div v-if="view === 'members'" class="space-y-3">
        <div v-if="loadingMembers" class="app-card p-6 text-center text-xs text-[#94A3B8]">
          正在加载成员列表…
        </div>

        <div v-else class="app-card divide-y divide-[#F1F3F6] overflow-hidden">
          <div v-if="members.length === 0" class="p-6 text-center text-xs text-[#94A3B8]">
            当前账本暂无其他成员
          </div>

          <div
            v-for="m in members"
            :key="m.user_id"
            class="px-4 py-3.5 flex items-center justify-between gap-3"
          >
            <div class="flex items-center gap-3 min-w-0">
              <img
                v-if="hasMemberAvatar(m)"
                :src="getMemberAvatar(m)"
                :alt="m.user_name"
                class="w-10 h-10 rounded-full object-cover bg-white border border-[#ECEEF2] shrink-0"
              />
              <span
                v-else
                class="w-10 h-10 rounded-full text-sm font-bold text-white flex items-center justify-center shrink-0"
                :style="{ backgroundColor: m.color || '#DE8A1E' }"
              >
                {{ getMemberInitial(m) }}
              </span>

              <div class="min-w-0">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="text-sm font-bold text-[#18181B] truncate">{{ m.user_name }}</span>
                  <span
                    v-if="m.is_me"
                    class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-[#F1F3F6] text-[#475569]"
                  >
                    我
                  </span>
                  <span
                    class="text-[10px] font-medium px-1.5 py-0.5 rounded-full"
                    :class="
                      m.status === 'invited'
                        ? 'bg-[#FFF6E8] text-[#DE8A1E]'
                        : 'bg-[#F1F3F6] text-[#64748B]'
                    "
                  >
                    {{ roleText(m) }}
                  </span>
                  <span
                    v-if="m.is_virtual"
                    class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-[#EEF2FF] text-[#4F46E5]"
                  >
                    虚拟成员
                  </span>
                </div>
                <p class="text-[11px] text-[#94A3B8] mt-0.5">
                  {{
                    m.status === 'invited'
                      ? '已发送邀请，等待对方加入确认'
                      : m.is_virtual
                        ? '无需登录账号，用于家庭分摊记账'
                        : '可同步查看与参与当前账本记账'
                  }}
                </p>
              </div>
            </div>

            <button
              v-if="!m.is_me && m.role !== 'owner'"
              type="button"
              :disabled="removingUserId === m.user_id"
              class="text-[11px] text-[#94A3B8] hover:text-[#D6553E] px-2 py-1 shrink-0 disabled:opacity-50 transition"
              @click="handleRemoveMember(m)"
            >
              {{ removingUserId === m.user_id ? '移除中…' : '移除' }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2.5 pt-1">
          <button
            type="button"
            class="py-3 rounded-2xl bg-white border border-[#E6E9EF] text-xs font-semibold text-[#18181B] hover:bg-[#F8F9FB] transition"
            @click="view = 'virtual'"
          >
            添加虚拟成员
          </button>
          <button
            type="button"
            class="py-3 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] transition"
            @click="view = 'invite'"
          >
            邀请成员加入
          </button>
        </div>
      </div>

      <!-- 态 2：邀请加入 -->
      <div v-else-if="view === 'invite'" class="space-y-3">
        <div class="app-card p-4 text-center">
          <img
            :src="DEFAULT_MASCOT"
            alt="七猫记账"
            class="w-12 h-12 rounded-2xl mx-auto mb-2 object-contain bg-white border border-[#ECEEF2] p-1"
            @error="onMascotError"
          />
          <p class="text-sm font-bold text-[#18181B]">
            邀请加入「{{ store.activeBook?.name || '当前账本' }}」
          </p>
          <p class="text-[11px] text-[#64748B] mt-0.5">
            对方通过 6 位邀请口令即可直接加入共同记账
          </p>

          <div
            class="mt-3.5 p-3 rounded-2xl bg-[#F5F6F8] border border-[#ECEEF2] flex items-center justify-between gap-2"
          >
            <div class="text-left min-w-0">
              <p class="text-[10px] text-[#94A3B8]">账本专属邀请口令</p>
              <p class="text-base font-bold tracking-wider text-[#18181B] tabular-nums mt-0.5 truncate">
                {{ inviteCodeText }}
              </p>
            </div>
            <button
              type="button"
              class="px-3 py-1.5 rounded-xl bg-white border border-[#E2E6EC] text-xs font-semibold text-[#DE8A1E] hover:bg-[#FFF6E8] transition shrink-0"
              @click="copyInviteCode"
            >
              复制口令
            </button>
          </div>
        </div>

        <div class="app-card p-4">
          <p class="text-xs font-bold text-[#18181B] mb-2.5">成员权限</p>
          <div class="grid grid-cols-2 gap-2">
            <button
              type="button"
              class="p-2.5 rounded-xl border text-left transition"
              :class="
                inviteRole === 'editor'
                  ? 'border-[#DE8A1E] bg-[#FFF6E8]'
                  : 'border-[#ECEEF2] bg-white'
              "
              @click="inviteRole = 'editor'"
            >
              <p class="text-xs font-bold text-[#18181B]">可记账与查看</p>
              <p class="text-[10px] text-[#64748B] mt-0.5">适合伴侣、家人共同记账</p>
            </button>

            <button
              type="button"
              class="p-2.5 rounded-xl border text-left transition"
              :class="
                inviteRole === 'viewer'
                  ? 'border-[#DE8A1E] bg-[#FFF6E8]'
                  : 'border-[#ECEEF2] bg-white'
              "
              @click="inviteRole = 'viewer'"
            >
              <p class="text-xs font-bold text-[#18181B]">仅查看统计</p>
              <p class="text-[10px] text-[#64748B] mt-0.5">可看报表，不可增删账单</p>
            </button>
          </div>
        </div>

        <div class="app-card p-4">
          <p class="text-xs font-bold text-[#18181B] mb-2">通过邮箱发送邀请</p>
          <form class="space-y-2.5" @submit.prevent="handleInviteByEmail">
            <input
              v-model="inviteEmail"
              type="email"
              required
              placeholder="输入对方邮箱，如 partner@example.com"
              class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 py-2.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
            <button
              type="submit"
              :disabled="inviting"
              class="w-full py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
            >
              {{ inviting ? '发送中…' : '发送邮件邀请' }}
            </button>
          </form>
        </div>
      </div>

      <!-- 态 3：添加虚拟成员 -->
      <div v-else class="app-card p-4">
        <p class="text-xs text-[#64748B] leading-relaxed mb-3.5">
          虚拟成员无需注册七猫记账账号，方便你在家庭账本中为长辈、孩子或宠物单独标记与分摊开销。
        </p>

        <form class="space-y-3.5" @submit.prevent="handleCreateVirtualMember">
          <div>
            <label class="block text-xs font-semibold text-[#18181B] mb-1.5">成员昵称</label>
            <input
              v-model="virtualName"
              type="text"
              required
              maxlength="12"
              placeholder="例如：妈妈、宝宝、团子"
              class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 py-2.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>

          <div>
            <label class="block text-xs font-semibold text-[#18181B] mb-2">专属头像色</label>
            <div class="flex items-center gap-2.5">
              <button
                v-for="color in LEDGER_COLOR_PRESETS"
                :key="color"
                type="button"
                class="w-7 h-7 rounded-full transition"
                :class="virtualColor === color ? 'ring-2 ring-offset-2 ring-[#18181B] scale-105' : ''"
                :style="{ backgroundColor: color }"
                @click="virtualColor = color"
              />
            </div>
          </div>

          <button
            type="submit"
            :disabled="addingVirtual"
            class="w-full py-2.5 rounded-xl bg-[#18181B] text-white text-xs font-semibold hover:bg-[#27272A] disabled:opacity-60 transition"
          >
            {{ addingVirtual ? '添加中…' : '确认添加' }}
          </button>
        </form>
      </div>
    </div>
  </BottomSheet>
</template>
