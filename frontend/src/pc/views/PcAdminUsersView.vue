<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { resolveAvatarUrl } from '@/constants/avatars';
import {
  normalizeRegConfig,
  normalizeAdminUser,
  type NormalizedAdminUser,
} from '@/pc/utils/adminAdapter';
import type { AdminRegConfig } from '@/types';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const store = useAppStore();

const regForm = reactive({
  allowRegister: true,
  requireInviteCode: false,
  requireEmailVerify: false,
  inviteCode: 'CAT7-NAS-2026',
  defaultDailyAiLimit: 30,
});
const savingReg = ref(false);

const searchQuery = ref('');
const loadingUsers = ref(false);
const users = ref<NormalizedAdminUser[]>([]);

const limitModalOpen = ref(false);
const editingUser = ref<NormalizedAdminUser | null>(null);
const limitInput = ref(30);
const savingUser = ref(false);

const filteredUsers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return users.value;
  return users.value.filter(
    (u) => u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q)
  );
});

async function loadRegConfig() {
  try {
    const raw = await api.getAdminRegConfig();
    const cfg = normalizeRegConfig(raw);
    regForm.allowRegister = cfg.allowRegister;
    regForm.requireInviteCode = cfg.requireInviteCode;
    regForm.requireEmailVerify = cfg.requireEmailVerify;
    regForm.inviteCode = cfg.inviteCode;
    regForm.defaultDailyAiLimit = cfg.defaultDailyAiLimit;
  } catch (err: any) {
    store.showToast(err?.message || '读取注册策略失败', 'error');
  }
}

async function loadUsers(search?: string) {
  loadingUsers.value = true;
  try {
    const list = await api.listAdminUsers(search);
    users.value = Array.isArray(list) ? list.map(normalizeAdminUser) : [];
  } catch (err: any) {
    store.showToast(err?.message || '加载用户列表失败', 'error');
  } finally {
    loadingUsers.value = false;
  }
}

onMounted(() => {
  loadRegConfig();
  loadUsers();
});

async function saveRegConfig(silent = false) {
  if (savingReg.value) return;
  savingReg.value = true;
  try {
    const payload: Record<string, any> = {
      allow_register: regForm.allowRegister,
      allow_public_register: regForm.allowRegister,
      require_invite_code: regForm.requireInviteCode,
      require_email_verify: regForm.requireEmailVerify,
      invite_code: regForm.inviteCode.trim() || 'CAT7-NAS-2026',
      default_daily_ai_limit: Math.max(1, Number(regForm.defaultDailyAiLimit) || 30),
    };
    const res = await api.updateAdminRegConfig(payload as Partial<AdminRegConfig>);
    const cfg = normalizeRegConfig(res);
    regForm.allowRegister = cfg.allowRegister;
    regForm.requireInviteCode = cfg.requireInviteCode;
    regForm.requireEmailVerify = cfg.requireEmailVerify;
    regForm.inviteCode = cfg.inviteCode;
    regForm.defaultDailyAiLimit = cfg.defaultDailyAiLimit;
    if (!silent) {
      store.showToast('注册与新用户默认策略已保存', 'success');
    } else {
      store.showToast('注册策略已更新', 'success');
    }
  } catch (err: any) {
    store.showToast(err?.message || '保存注册策略失败', 'error');
  } finally {
    savingReg.value = false;
  }
}

function generateInviteCode() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  let suffix = '';
  for (let i = 0; i < 4; i++) {
    suffix += chars[Math.floor(Math.random() * chars.length)];
  }
  regForm.inviteCode = `CAT7-${suffix}`;
}

function isCurrentSelf(u: NormalizedAdminUser): boolean {
  return Boolean(store.user?.user_id && u.id === store.user.user_id);
}

async function handleToggleUserStatus(u: NormalizedAdminUser, nextActive: boolean) {
  if (isCurrentSelf(u)) {
    store.showToast('不能禁用当前登录管理员账号', 'error');
    return;
  }
  try {
    await api.updateAdminUser(u.id, { is_active: nextActive });
    await loadUsers(searchQuery.value.trim() || undefined);
    store.showToast(`「${u.name}」已${nextActive ? '启用' : '禁用'}`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新用户状态失败', 'error');
  }
}

async function handleToggleUserRole(u: NormalizedAdminUser) {
  if (isCurrentSelf(u)) {
    store.showToast('不能修改当前登录管理员自身角色', 'error');
    return;
  }
  const nextRole = u.role === 'admin' ? 'member' : 'admin';
  try {
    await api.updateAdminUser(u.id, { role: nextRole });
    await loadUsers(searchQuery.value.trim() || undefined);
    store.showToast(
      `已将「${u.name}」切换为${nextRole === 'admin' ? '管理员' : '普通成员'}`,
      'success'
    );
  } catch (err: any) {
    store.showToast(err?.message || '切换用户角色失败', 'error');
  }
}

function openEditLimit(u: NormalizedAdminUser) {
  editingUser.value = u;
  limitInput.value = u.dailyAiLimit;
  limitModalOpen.value = true;
}

async function handleSaveUserLimit() {
  if (!editingUser.value || savingUser.value) return;
  savingUser.value = true;
  try {
    const val = Math.max(0, Math.round(Number(limitInput.value) || 0));
    await api.updateAdminUser(editingUser.value.id, { daily_ai_limit: val });
    limitModalOpen.value = false;
    await loadUsers(searchQuery.value.trim() || undefined);
    store.showToast(`已将「${editingUser.value.name}」日限额更新为 ${val} 次`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '修改限额失败', 'error');
  } finally {
    savingUser.value = false;
  }
}
</script>

<template>
  <div class="space-y-5">
    <!-- 注册策略配置卡 -->
    <section class="pc-card p-5 space-y-4 text-xs">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">注册策略与新用户配额</h2>
          <p class="text-[11px] text-[#64748B] mt-0.5">
            控制外部用户自助注册、邀请码准入、邮箱验证码校验与默认 AI 日限额
          </p>
        </div>
        <button
          type="button"
          :disabled="savingReg"
          class="btn-amber px-4 py-2 rounded-xl text-white font-semibold disabled:opacity-60 transition"
          @click="saveRegConfig(false)"
        >
          {{ savingReg ? '保存中…' : '保存注册策略' }}
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex items-center justify-between gap-3">
          <div>
            <p class="font-bold text-[#18181B]">开放公开注册</p>
            <p class="text-[10px] text-[#94A3B8] mt-0.5">关闭后新用户将无法自助注册</p>
          </div>
          <ToggleSwitch
            v-model="regForm.allowRegister"
            aria-label="开放公开注册"
            @change="saveRegConfig(true)"
          />
        </div>

        <div class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex items-center justify-between gap-3">
          <div>
            <p class="font-bold text-[#18181B]">强制邮箱验证</p>
            <p class="text-[10px] text-[#94A3B8] mt-0.5">注册时需输入 6 位邮箱验证码</p>
          </div>
          <ToggleSwitch
            v-model="regForm.requireEmailVerify"
            aria-label="强制邮箱验证"
            @change="saveRegConfig(true)"
          />
        </div>

        <div class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex items-center justify-between gap-3">
          <div>
            <p class="font-bold text-[#18181B]">仅限邀请码注册</p>
            <p class="text-[10px] text-[#94A3B8] mt-0.5">开启后注册须校验下方系统邀请码</p>
          </div>
          <ToggleSwitch
            v-model="regForm.requireInviteCode"
            aria-label="仅限邀请码注册"
            @change="saveRegConfig(true)"
          />
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">系统注册邀请码</label>
          <div class="flex gap-2">
            <input
              v-model="regForm.inviteCode"
              type="text"
              maxlength="32"
              placeholder="如：CAT7-NAS-2026"
              class="flex-1 min-w-0 h-9 font-mono bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
            <button
              type="button"
              class="h-9 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#475569] shrink-0"
              @click="generateInviteCode"
            >
              随机生成
            </button>
          </div>
        </div>

        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">
            新用户默认 AI 日限额（次/天）
          </label>
          <input
            v-model.number="regForm.defaultDailyAiLimit"
            type="number"
            min="1"
            max="9999"
            class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
        </div>
      </div>
    </section>

    <!-- 成员列表管理（零私人账单明细） -->
    <section class="pc-card overflow-hidden">
      <div class="px-5 py-3.5 border-b border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">系统成员列表 ({{ users.length }})</h2>
          <p class="text-[11px] text-[#94A3B8]">
            支持角色权限切换、账号启禁用与个人 AI 配额管理
          </p>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="搜索成员昵称或邮箱…"
            class="w-52 h-8 text-xs bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            @keyup.enter="loadUsers(searchQuery.trim() || undefined)"
          />
          <button
            type="button"
            class="h-8 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#475569]"
            @click="loadUsers(searchQuery.trim() || undefined)"
          >
            {{ loadingUsers ? '查询中…' : '搜索' }}
          </button>
        </div>
      </div>

      <div v-if="filteredUsers.length === 0" class="py-10 text-center text-xs text-[#94A3B8]">
        暂无匹配的成员记录
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
              <th class="py-2.5 px-5 font-semibold">成员</th>
              <th class="py-2.5 px-4 font-semibold">角色</th>
              <th class="py-2.5 px-4 font-semibold text-right">账本数</th>
              <th class="py-2.5 px-4 font-semibold text-right">记账总数</th>
              <th class="py-2.5 px-4 font-semibold text-right">AI 日限额</th>
              <th class="py-2.5 px-4 font-semibold">状态</th>
              <th class="py-2.5 px-5 font-semibold text-right">管理操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#F1F3F6]">
            <tr v-for="u in filteredUsers" :key="u.id" class="hover:bg-[#FAFBFD] transition">
              <td class="py-3 px-5">
                <div class="flex items-center gap-2.5">
                  <img
                    :src="resolveAvatarUrl(u.avatar)"
                    :alt="u.name"
                    class="w-8 h-8 rounded-full object-cover border border-[#EAECEF] bg-white shrink-0"
                  />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <p class="font-bold text-[#18181B] truncate">{{ u.name }}</p>
                      <span
                        v-if="isCurrentSelf(u)"
                        class="text-[9px] px-1.5 py-0.5 rounded bg-[#FFF6E8] text-[#DE8A1E] font-semibold"
                      >
                        当前账号
                      </span>
                    </div>
                    <p class="text-[10px] text-[#94A3B8] truncate">{{ u.email || '未绑定邮箱' }}</p>
                  </div>
                </div>
              </td>
              <td class="py-3 px-4">
                <span
                  class="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                  :class="
                    u.role === 'admin'
                      ? 'bg-[#18181B] text-white'
                      : 'bg-[#F4F5F8] text-[#475569]'
                  "
                >
                  {{ u.role === 'admin' ? '管理员' : '普通成员' }}
                </span>
              </td>
              <td class="py-3 px-4 text-right tabular-nums text-[#64748B]">
                {{ u.booksCount }} 本
              </td>
              <td class="py-3 px-4 text-right tabular-nums font-semibold">
                {{ u.txCount.toLocaleString() }} 笔
              </td>
              <td class="py-3 px-4 text-right tabular-nums font-semibold text-[#DE8A1E]">
                {{ u.todayAiCount }}/{{ u.dailyAiLimit }} 次
              </td>
              <td class="py-3 px-4">
                <div class="flex items-center gap-2">
                  <ToggleSwitch
                    :model-value="u.isActive"
                    :disabled="isCurrentSelf(u)"
                    aria-label="启禁用用户账号"
                    @change="handleToggleUserStatus(u, $event)"
                  />
                  <span
                    class="text-[10px] font-semibold"
                    :class="u.isActive ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                  >
                    {{ u.isActive ? '正常' : '已禁用' }}
                  </span>
                </div>
              </td>
              <td class="py-3 px-5 text-right space-x-2.5 whitespace-nowrap">
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
                  @click="openEditLimit(u)"
                >
                  改限额
                </button>
                <button
                  v-if="!isCurrentSelf(u)"
                  type="button"
                  class="text-[11px] font-semibold text-[#475569] hover:text-[#18181B] hover:underline"
                  @click="handleToggleUserRole(u)"
                >
                  {{ u.role === 'admin' ? '降为成员' : '设为管理员' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 修改单用户限额弹窗 -->
    <Teleport to="body">
      <div v-if="limitModalOpen" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
          @click="limitModalOpen = false"
        />
        <div class="relative w-full max-w-[380px] bg-white rounded-[22px] border border-[#EAECEF] shadow-2xl p-5 space-y-4">
          <h3 class="text-sm font-bold text-[#18181B]">
            设置「{{ editingUser?.name }}」每日 AI 次数上限
          </h3>
          <form class="space-y-3 text-xs" @submit.prevent="handleSaveUserLimit">
            <input
              v-model.number="limitInput"
              type="number"
              min="0"
              max="9999"
              required
              class="w-full h-10 text-base font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
            <div class="flex justify-end gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-[#475569] font-semibold"
                @click="limitModalOpen = false"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingUser"
                class="btn-amber px-4 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingUser ? '保存中…' : '确认保存' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>
