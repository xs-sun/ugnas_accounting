<script setup lang="ts">
import { ref, computed } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { resolveAvatarUrl } from '@/constants/avatars';
import { formatMoney } from '@/utils/format';
import type { InviteSheetView, SubpageType } from '../types';

const emit = defineEmits<{
  'open-ledger': [];
  'open-invite': [view?: InviteSheetView];
  'open-subpage': [page: SubpageType];
  'open-auth': [view: 'login' | 'register' | 'forgot'];
}>();

const store = useAppStore();

// 修改昵称内联状态
const editingName = ref(false);
const nameDraft = ref('');
const savingName = ref(false);
const loggingOut = ref(false);

const userAvatar = computed(() => resolveAvatarUrl(store.user?.avatar));

const roleBadge = computed(() => {
  const role = store.user?.role || store.user?.user_type || '';
  if (role === 'admin') {
    return { label: '管理员', cls: 'bg-[#FFF6E8] text-[#DE8A1E]' };
  }
  if (role === 'guest') {
    return { label: 'NAS 访客', cls: 'bg-[#F1F5F9] text-[#64748B]' };
  }
  return { label: '七猫会员', cls: 'bg-[#ECF6F0] text-[#3E8E63]' };
});

const emailBound = computed(() => Boolean(store.user?.email_bound && store.user?.email));

const recordedDays = computed(() => store.user?.recorded_days ?? 0);
const totalTxCount = computed(
  () => store.user?.total_transactions_count ?? store.transactionsTotal ?? store.transactions.length
);

// 财务健康度与预算执行率/结余率计算
const healthScore = computed(() => {
  if (typeof store.user?.health_score === 'number' && store.user.health_score > 0) {
    return Math.min(100, Math.max(0, Math.round(store.user.health_score)));
  }
  const budPct = store.budgets?.percentage ?? 0;
  const base = budPct > 100 ? 64 : budPct > 85 ? 76 : 86;
  return base;
});

const healthLabel = computed(() => {
  if (store.user?.health_label) return store.user.health_label;
  if (healthScore.value >= 85) return '稳健';
  if (healthScore.value >= 70) return '良好';
  return '需关注';
});

const healthGrade = computed(() => {
  if (healthScore.value >= 85) return '优';
  if (healthScore.value >= 70) return '良';
  return '平';
});

const savingsRate = computed(() => {
  if (typeof store.user?.savings_rate === 'number') {
    return Math.round(store.user.savings_rate);
  }
  if (typeof store.stats?.savings_rate === 'number') {
    return Math.round(store.stats.savings_rate);
  }
  return 0;
});

const budgetExecRate = computed(() => Math.round(store.budgets?.percentage ?? 0));

const ringCircumference = 2 * Math.PI * 25; // ~157.08
const healthDashArray = computed(() => {
  const filled = (healthScore.value / 100) * ringCircumference;
  return `${filled.toFixed(1)} ${ringCircumference.toFixed(1)}`;
});

const activeRecurringCount = computed(
  () => store.recurringRules.filter((r) => r.is_active).length
);

const budgetSubText = computed(() => {
  const limit = store.budgets?.total_budget ?? store.activeBook?.budget ?? 0;
  if (!limit) return '去设定月预算';
  return `月限${formatMoney(limit, { privacy: store.privacyHidden, compact: true, decimals: 0 })}`;
});

function handleAvatarClick() {
  store.openAvatarModal(
    'profile',
    async (avatar) => {
      try {
        const updated = await api.updateProfile({ avatar });
        store.user = updated;
      } catch {
        if (store.user) {
          store.user = { ...store.user, avatar };
        }
      }
    },
    store.user?.avatar || ''
  );
}

function startEditName() {
  nameDraft.value = store.user?.user_name || '';
  editingName.value = true;
}

async function saveProfileName() {
  const nextName = nameDraft.value.trim();
  if (!nextName) {
    store.showToast('昵称不能为空', 'error');
    return;
  }
  if (savingName.value) return;
  savingName.value = true;
  try {
    const updated = await api.updateProfile({ name: nextName });
    store.user = updated;
    editingName.value = false;
    store.showToast('昵称已更新', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '修改昵称失败', 'error');
  } finally {
    savingName.value = false;
  }
}

async function handleLogout() {
  if (loggingOut.value) return;
  loggingOut.value = true;
  try {
    await store.logoutToGuest();
    store.showToast(
      store.authRequired ? '已退出登录，请重新登录' : '已退出登录，恢复默认 NAS 身份',
      'info'
    );
  } catch (err: any) {
    store.showToast(err?.message || '退出登录失败', 'error');
  } finally {
    loggingOut.value = false;
  }
}
</script>

<template>
  <div class="space-y-3.5">
    <!-- 顶部用户资料卡片 -->
    <section class="app-card p-4">
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-3 min-w-0">
          <button
            type="button"
            title="点击更换头像"
            class="relative shrink-0 group"
            @click="handleAvatarClick"
          >
            <img
              :src="userAvatar"
              :alt="store.user?.user_name || '用户头像'"
              class="w-12 h-12 rounded-full object-cover bg-white border border-[#ECEEF2] shadow-xs group-hover:ring-2 group-hover:ring-[#DE8A1E]/40 transition"
            />
            <span
              class="absolute -bottom-0.5 -right-0.5 w-5 h-5 rounded-full bg-[#18181B] text-white border-2 border-white flex items-center justify-center"
            >
              <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z" />
                <circle cx="12" cy="13" r="3.5" />
              </svg>
            </span>
          </button>

          <div class="min-w-0">
            <div class="flex items-center gap-1.5 flex-wrap">
              <h2 class="text-[15px] font-bold text-[#18181B] truncate">
                {{ store.user?.user_name || '七猫记账用户' }}
              </h2>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full shrink-0"
                :class="roleBadge.cls"
              >
                {{ roleBadge.label }}
              </span>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full shrink-0"
                :class="emailBound ? 'bg-[#ECF6F0] text-[#3E8E63]' : 'bg-[#F5F6F8] text-[#94A3B8]'"
              >
                {{ emailBound ? '已认证' : '未认证' }}
              </span>
            </div>
            <p class="text-[11px] text-[#64748B] truncate mt-0.5">
              {{ store.user?.username ? `@${store.user.username} · ` : '' }}{{ emailBound ? store.user?.email : '暂未绑定安全邮箱' }}
            </p>
          </div>
        </div>

        <button
          type="button"
          class="shrink-0 text-[11px] font-semibold px-3 py-1.5 rounded-full bg-[#F5F6F8] text-[#18181B] hover:bg-[#ECEEF2] transition"
          @click="emit('open-ledger')"
        >
          切换账本
        </button>
      </div>

      <!-- 3 项核心指标 -->
      <div class="grid grid-cols-3 divide-x divide-[#F1F3F6] mt-4 pt-3.5 border-t border-[#F1F3F6] text-center">
        <div class="px-1">
          <p class="text-base font-bold text-[#18181B] tabular-nums">{{ recordedDays }}</p>
          <p class="text-[10px] text-[#94A3B8] mt-0.5">记账天数</p>
        </div>
        <div class="px-1">
          <p class="text-base font-bold text-[#18181B] tabular-nums">{{ totalTxCount }}</p>
          <p class="text-[10px] text-[#94A3B8] mt-0.5">累计笔数</p>
        </div>
        <div class="px-1">
          <p class="text-base font-bold text-[#18181B] tabular-nums truncate">
            {{ formatMoney(store.walletSummary.net_assets, { privacy: store.privacyHidden, compact: true }) }}
          </p>
          <p class="text-[10px] text-[#94A3B8] mt-0.5">净资产</p>
        </div>
      </div>
    </section>

    <!-- 财务健康度圆环卡 -->
    <section class="app-card p-4 flex items-center justify-between gap-3">
      <div class="min-w-0">
        <p class="text-[11px] font-semibold text-[#DE8A1E] mb-1">
          {{ store.currentMonth }}月财务健康度
        </p>
        <div class="flex items-baseline gap-1.5">
          <span class="text-2xl font-bold text-[#18181B] tabular-nums">{{ healthScore }}</span>
          <span class="text-xs text-[#64748B]">/ 100 · {{ healthLabel }}</span>
        </div>
        <p class="text-[11px] text-[#64748B] mt-1 tabular-nums">
          预算执行率 {{ budgetExecRate }}% · 结余率 {{ savingsRate }}%
        </p>
      </div>

      <div class="relative w-14 h-14 shrink-0">
        <svg viewBox="0 0 64 64" class="w-full h-full -rotate-90">
          <circle cx="32" cy="32" r="25" fill="none" stroke="#ECEEF2" stroke-width="5.5" />
          <circle
            cx="32"
            cy="32"
            r="25"
            fill="none"
            stroke="#DE8A1E"
            stroke-width="5.5"
            stroke-linecap="round"
            :stroke-dasharray="healthDashArray"
            class="transition-all duration-300"
          />
        </svg>
        <span class="absolute inset-0 flex items-center justify-center text-xs font-bold text-[#18181B]">
          {{ healthGrade }}
        </span>
      </div>
    </section>

    <!-- 6 宫格账务管理快捷入口 -->
    <section class="app-card p-4">
      <div class="flex items-center justify-between mb-3.5">
        <h3 class="text-xs font-bold text-[#18181B] tracking-tight">账务管理</h3>
        <span class="text-[10px] text-[#94A3B8]">共 6 项功能</span>
      </div>

      <div class="grid grid-cols-3 gap-y-4 gap-x-2">
        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'new-ledger')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M5 4.5h11.5A2.5 2.5 0 0 1 19 7v12.5H6.5A1.5 1.5 0 0 1 5 18V4.5z" />
              <path d="M9.5 11.5h5M12 9v5" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">账本管理</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight">
            共{{ store.books.length }}本
          </span>
        </button>

        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'budgets')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="8" />
              <path d="M12 12l4.5-3M12 8v4" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">预算管理</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight truncate max-w-full px-1">
            {{ budgetSubText }}
          </span>
        </button>

        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'wallets')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3.5" y="6" width="17" height="12" rx="3" />
              <path d="M3.5 10.5h17M7.5 14.5h3.5" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">资产账户</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight">
            {{ store.activeWallets.length }}个账户
          </span>
        </button>

        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'categories')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <rect x="4" y="4" width="6.5" height="6.5" rx="1.8" />
              <rect x="13.5" y="4" width="6.5" height="6.5" rx="1.8" />
              <rect x="4" y="13.5" width="6.5" height="6.5" rx="1.8" />
              <path d="M16.75 13.5v6.5M13.5 16.75h6.5" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">分类管理</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight">支持二级</span>
        </button>

        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'recurring')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="8" />
              <path d="M12 7.5V12l3 2" />
              <path d="M16.5 4.2l2.5 2" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">定时记账</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight">
            {{ activeRecurringCount }}条执行中
          </span>
        </button>

        <button
          type="button"
          class="group flex flex-col items-center text-center py-1.5 rounded-2xl hover:bg-[#F5F6F8] transition"
          @click="emit('open-subpage', 'import-export')"
        >
          <div
            class="w-10 h-10 rounded-[14px] bg-[#F5F6F8] group-hover:bg-[#FFF6E8] text-[#18181B] group-hover:text-[#DE8A1E] flex items-center justify-center transition mb-1.5"
          >
            <svg class="w-[19px] h-[19px]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 3v12m0 0l-4-4m4 4l4-4" />
              <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
            </svg>
          </div>
          <span class="text-[12px] font-semibold text-[#18181B] leading-tight">导入导出</span>
          <span class="text-[10px] text-[#94A3B8] mt-0.5 leading-tight">微信/支付宝</span>
        </button>
      </div>
    </section>

    <!-- 账户与安全分组列表 -->
    <section class="app-card divide-y divide-[#F1F3F6] overflow-hidden">
      <!-- 修改昵称 -->
      <div class="px-4 py-3.5">
        <button
          v-if="!editingName"
          type="button"
          class="w-full flex items-center justify-between hover:opacity-85 transition text-left"
          @click="startEditName"
        >
          <div class="flex items-center gap-3 min-w-0">
            <svg class="w-[18px] h-[18px] text-[#475569] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="8" r="4" />
              <path d="M5 20c0-3.3 3.1-5.5 7-5.5s7 2.2 7 5.5" />
            </svg>
            <span class="text-xs font-semibold text-[#18181B]">修改昵称</span>
          </div>
          <div class="flex items-center gap-1.5 shrink-0">
            <span class="text-[11px] text-[#64748B] truncate max-w-[120px]">
              {{ store.user?.user_name || '未设置' }}
            </span>
            <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
          </div>
        </button>

        <form v-else class="flex items-center gap-2" @submit.prevent="saveProfileName">
          <input
            v-model="nameDraft"
            type="text"
            maxlength="20"
            placeholder="请输入新昵称"
            class="flex-1 min-w-0 h-8 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
          <button
            type="button"
            class="px-2.5 h-8 rounded-xl bg-[#F5F6F8] text-[11px] font-semibold text-[#64748B]"
            @click="editingName = false"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="savingName"
            class="px-3 h-8 rounded-xl bg-[#DE8A1E] text-white text-[11px] font-semibold disabled:opacity-60"
          >
            {{ savingName ? '保存中' : '保存' }}
          </button>
        </form>
      </div>

      <!-- 绑定/更换邮箱 -->
      <button
        type="button"
        class="w-full px-4 py-3.5 flex items-center justify-between hover:bg-[#F8F9FB] transition text-left"
        @click="emit('open-subpage', 'bind-email')"
      >
        <div class="flex items-center gap-3 min-w-0">
          <svg class="w-[18px] h-[18px] text-[#475569] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="5" width="18" height="14" rx="2.5" />
            <path d="M3.5 7l8.5 6 8.5-6" />
          </svg>
          <span class="text-xs font-semibold text-[#18181B]">
            {{ emailBound ? '更换绑定邮箱' : '绑定安全邮箱' }}
          </span>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <span
            class="text-[11px] truncate max-w-[140px]"
            :class="emailBound ? 'text-[#64748B]' : 'text-[#DE8A1E] font-medium'"
          >
            {{ emailBound ? store.user?.email : '去绑定' }}
          </span>
          <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M9 18l6-6-6-6" />
          </svg>
        </div>
      </button>

      <!-- 账本成员管理 -->
      <button
        type="button"
        class="w-full px-4 py-3.5 flex items-center justify-between hover:bg-[#F8F9FB] transition text-left"
        @click="emit('open-invite', 'members')"
      >
        <div class="flex items-center gap-3 min-w-0">
          <svg class="w-[18px] h-[18px] text-[#475569] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="9" cy="8.5" r="3" />
            <path d="M3.5 18.5c0-2.8 2.5-4.8 5.5-4.8s5.5 2 5.5 4.8" />
            <path d="M17.5 9.5v5M15 12h5" />
          </svg>
          <span class="text-xs font-semibold text-[#18181B]">成员协作与邀请</span>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <span class="text-[11px] text-[#64748B]">
            {{ store.activeBook?.members?.length || 1 }} 位成员
          </span>
          <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M9 18l6-6-6-6" />
          </svg>
        </div>
      </button>

      <!-- 数据导入与导出 -->
      <button
        type="button"
        class="w-full px-4 py-3.5 flex items-center justify-between hover:bg-[#F8F9FB] transition text-left"
        @click="emit('open-subpage', 'import-export')"
      >
        <div class="flex items-center gap-3">
          <svg class="w-[18px] h-[18px] text-[#475569]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 3v12m0 0l-4-4m4 4l4-4" />
            <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
          </svg>
          <span class="text-xs font-semibold text-[#18181B]">数据导入 / 导出 CSV</span>
        </div>
        <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M9 18l6-6-6-6" />
        </svg>
      </button>

      <!-- 登录 / 注册 / 切换账号 -->
      <button
        type="button"
        class="w-full px-4 py-3.5 flex items-center justify-between hover:bg-[#F8F9FB] transition text-left"
        @click="emit('open-auth', 'login')"
      >
        <div class="flex items-center gap-3">
          <svg class="w-[18px] h-[18px] text-[#DE8A1E]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
            <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
            <polyline points="10 17 15 12 10 7" />
            <line x1="15" y1="12" x2="3" y2="12" />
          </svg>
          <span class="text-xs font-semibold text-[#DE8A1E]">
            {{ store.hasAuthToken() ? '切换账号 / 注册新号' : '登录 / 注册账号' }}
          </span>
        </div>
        <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M9 18l6-6-6-6" />
        </svg>
      </button>

      <!-- 退出当前账号登录 -->
      <button
        v-if="store.isAuthenticated"
        type="button"
        :disabled="loggingOut"
        class="w-full px-4 py-3.5 flex items-center justify-between hover:bg-[#FDF0EE]/60 transition text-left disabled:opacity-60"
        @click="handleLogout"
      >
        <div class="flex items-center gap-3">
          <svg class="w-[18px] h-[18px] text-[#D6553E]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <path d="M16 17l5-5-5-5M21 12H9" />
          </svg>
          <span class="text-xs font-semibold text-[#D6553E]">
            {{ loggingOut ? '正在退出…' : '退出登录' }}
          </span>
        </div>
        <svg class="w-4 h-4 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M9 18l6-6-6-6" />
        </svg>
      </button>
    </section>
  </div>
</template>
