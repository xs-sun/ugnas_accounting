<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue';
import { api } from '@/api/client';
import { DEFAULT_MASCOT, MASCOT_FALLBACK } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import type { Transaction } from '@/types';
import type { InviteSheetView, MobileTab, RecordInitialMode, SubpageType } from './types';

import MobileRecordsView from './views/MobileRecordsView.vue';
import MobileCalendarView from './views/MobileCalendarView.vue';
import MobileStatsView from './views/MobileStatsView.vue';
import MobileProfileView from './views/MobileProfileView.vue';

import MobileRecordSheet from './sheets/MobileRecordSheet.vue';
import MobileLedgerSheet from './sheets/MobileLedgerSheet.vue';
import MobileInviteSheet from './sheets/MobileInviteSheet.vue';
import MobileDetailSheet from './sheets/MobileDetailSheet.vue';
import MobileSubpageSheet from './sheets/MobileSubpageSheet.vue';
import MobileAuthScreen from './sheets/MobileAuthScreen.vue';

const store = useAppStore();

// 主导航 Tab
const activeTab = ref<MobileTab>('records');

// 顶部悬浮搜索状态
const searchOpen = ref(false);
const searchQuery = ref('');
const searchInputRef = ref<HTMLInputElement | null>(null);

// 记一笔抽屉状态
const recordSheetOpen = ref(false);
const recordInitialMode = ref<RecordInitialMode>('ai');
const recordPrefillDate = ref('');

// 账本抽屉状态
const ledgerSheetOpen = ref(false);

// 成员邀请抽屉状态
const inviteSheetOpen = ref(false);
const inviteInitialView = ref<InviteSheetView>('members');

// 流水详情 / 快捷月预算抽屉状态
const detailSheetOpen = ref(false);
const detailMode = ref<'transaction' | 'budget'>('transaction');
const selectedTransaction = ref<Transaction | null>(null);

// 全屏管理子页状态
const subpageSheetOpen = ref(false);
const activeSubpage = ref<SubpageType>('new-ledger');

// 全屏认证页状态
const authScreenOpen = ref(false);
const authInitialView = ref<'login' | 'register' | 'forgot'>('login');

const showAuthScreen = computed(
  () => authScreenOpen.value || (store.initialized && store.authRequired)
);

const activeBookMembers = computed(() => store.activeBook?.members || []);

const anyOverlayOpen = computed(
  () =>
    recordSheetOpen.value ||
    ledgerSheetOpen.value ||
    inviteSheetOpen.value ||
    detailSheetOpen.value ||
    subpageSheetOpen.value ||
    showAuthScreen.value
);

const mainPaddingStyle = computed(() => ({
  paddingTop: searchOpen.value
    ? 'calc(env(safe-area-inset-top, 0px) + 112px)'
    : 'calc(env(safe-area-inset-top, 0px) + 64px)',
}));

async function toggleSearch() {
  searchOpen.value = !searchOpen.value;
  if (searchOpen.value) {
    activeTab.value = 'records';
    await nextTick();
    searchInputRef.value?.focus();
  } else {
    searchQuery.value = '';
  }
}

function clearSearch() {
  searchQuery.value = '';
  searchInputRef.value?.focus();
}

function handleTogglePrivacy() {
  store.togglePrivacy();
  store.showToast(
    store.privacyHidden ? '已开启金额隐私隐藏' : '已恢复显示金额',
    'info'
  );
}

function openRecordSheet(mode: RecordInitialMode = 'ai', dateStr = '') {
  recordInitialMode.value = mode;
  recordPrefillDate.value = dateStr;
  recordSheetOpen.value = true;
}

function openRecordForDate(isoDate: string) {
  openRecordSheet('expense', isoDate);
}

function openInviteSheet(view: InviteSheetView = 'members') {
  inviteInitialView.value = view;
  inviteSheetOpen.value = true;
}

function openTxDetail(tx: Transaction) {
  selectedTransaction.value = tx;
  detailMode.value = 'transaction';
  detailSheetOpen.value = true;
}

async function openTxById(txId: string) {
  const local = store.transactions.find((t) => t.id === txId);
  if (local) {
    openTxDetail(local);
    return;
  }
  try {
    const fetched = await api.getTransaction(txId);
    openTxDetail(fetched);
  } catch (err: any) {
    store.showToast(err?.message || '未找到该笔账单详情', 'error');
  }
}

function openBudgetDetail() {
  selectedTransaction.value = null;
  detailMode.value = 'budget';
  detailSheetOpen.value = true;
}

function openSubpageSheet(page: SubpageType) {
  activeSubpage.value = page;
  subpageSheetOpen.value = true;
}

function openAuthScreen(view: 'login' | 'register' | 'forgot' = 'login') {
  authInitialView.value = view;
  authScreenOpen.value = true;
}

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

onMounted(() => {
  if (!store.initialized && !store.loading) {
    void store.initApp();
  }
});
</script>

<template>
  <div class="relative w-full h-screen h-[100dvh] bg-[#F5F6F8] flex flex-col overflow-hidden text-[#18181B]">
    <!-- 顶部悬浮双毛玻璃胶囊栏 -->
    <div
      v-if="!store.authRequired"
      class="fixed top-0 left-0 right-0 z-20 top-fade-mask pt-safe pb-3 pointer-events-none"
    >
      <header class="px-4 pt-2.5 flex items-center justify-between gap-2 pointer-events-auto">
        <!-- 左侧：当前账本名 + 成员圆点 + 下拉箭头 -->
        <button
          type="button"
          class="glass-top-pill flex items-center gap-2 pl-2.5 pr-3 py-1.5 rounded-full hover:bg-white transition min-w-0 max-w-[66%]"
          @click="ledgerSheetOpen = true"
        >
          <span
            class="w-2.5 h-2.5 rounded-full shrink-0"
            :style="{ backgroundColor: store.activeBook?.color || '#DE8A1E' }"
          />
          <span class="text-[13px] font-semibold text-[#18181B] truncate">
            {{ store.activeBook?.name || '七猫账本' }}
          </span>
          <span
            v-if="activeBookMembers.length > 1"
            class="flex items-center -space-x-1 shrink-0"
            :title="`共 ${activeBookMembers.length} 位成员`"
          >
            <span
              v-for="m in activeBookMembers.slice(0, 3)"
              :key="m.user_id"
              class="w-2.5 h-2.5 rounded-full ring-1 ring-white"
              :style="{ backgroundColor: m.color || '#3E8E63' }"
            />
          </span>
          <svg
            class="w-3.5 h-3.5 text-[#64748B] shrink-0"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
          >
            <path d="M6 9l6 6 6-6" />
          </svg>
        </button>

        <!-- 右侧：搜索与隐私眼睛胶囊 -->
        <div class="glass-top-pill flex items-center gap-0.5 p-1 rounded-full shrink-0">
          <button
            type="button"
            title="搜索账单"
            aria-label="搜索账单"
            class="w-7 h-7 rounded-full flex items-center justify-center transition"
            :class="
              searchOpen
                ? 'bg-[#FFF6E8] text-[#DE8A1E]'
                : 'text-[#475569] hover:bg-[#FFF6E8] hover:text-[#DE8A1E]'
            "
            @click="toggleSearch"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" />
            </svg>
          </button>

          <button
            type="button"
            title="隐藏或显示金额"
            aria-label="切换隐私显示"
            class="w-7 h-7 rounded-full flex items-center justify-center text-[#475569] hover:bg-[#FFF6E8] hover:text-[#DE8A1E] transition"
            @click="handleTogglePrivacy"
          >
            <svg
              v-if="!store.privacyHidden"
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
            <svg
              v-else
              class="w-4 h-4 text-[#DE8A1E]"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M17.94 17.94A10.94 10.94 0 0 1 12 19c-6.5 0-10-7-10-7a21.8 21.8 0 0 1 5.06-5.94" />
              <path d="M1 1l22 22" />
              <path d="M9.9 4.24A9.12 9.12 0 0 1 12 5c6.5 0 10 7 10 7a21.75 21.75 0 0 1-3.17 4.35" />
            </svg>
          </button>
        </div>
      </header>

      <!-- 可展开悬浮实时搜索条 -->
      <div v-if="searchOpen" class="px-4 pt-2 pointer-events-auto">
        <div class="glass-top-pill flex items-center gap-2 rounded-full px-3.5 py-2">
          <svg class="w-4 h-4 text-[#94A3B8] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="7" />
            <path d="M20 20l-3.5-3.5" />
          </svg>
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="text"
            placeholder="搜索商户、备注或分类（如：咖啡、盒马）"
            class="w-full text-xs bg-transparent outline-none text-[#18181B] placeholder-[#94A3B8]"
          />
          <button
            v-if="searchQuery"
            type="button"
            class="text-[11px] text-[#64748B] hover:text-[#18181B] px-1 shrink-0"
            @click="clearSearch"
          >
            清除
          </button>
        </div>
      </div>
    </div>

    <!-- 主内容滚动区 -->
    <main
      class="flex-1 min-h-0 phone-scroll px-4 pb-28 space-y-3.5"
      :class="
        anyOverlayOpen
          ? 'overflow-hidden pointer-events-none touch-none'
          : 'overflow-y-auto'
      "
      :style="mainPaddingStyle"
    >
      <div
        v-if="!store.initialized && store.loading"
        class="app-card p-7 text-center space-y-3"
      >
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫记账"
          class="w-12 h-12 rounded-2xl mx-auto object-contain bg-white p-1 border border-[#ECEEF2] animate-pulse"
          @error="onMascotError"
        />
        <p class="text-xs font-medium text-[#64748B]">正在校验登录状态与同步账本数据…</p>
      </div>

      <div
        v-else-if="store.authRequired"
        class="app-card p-6 text-center space-y-3"
      >
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫记账"
          class="w-12 h-12 rounded-2xl mx-auto object-contain bg-white p-1 border border-[#ECEEF2]"
          @error="onMascotError"
        />
        <div class="space-y-1">
          <p class="text-sm font-bold text-[#18181B]">当前处于未登录状态</p>
          <p class="text-xs text-[#64748B]">未检测到 NAS 网关登录信息，请登录后查看并管理账本</p>
        </div>
        <button
          type="button"
          class="btn-amber px-5 py-2.5 rounded-full text-white text-xs font-semibold"
          @click="openAuthScreen('login')"
        >
          前往登录 / 注册
        </button>
      </div>

      <div
        v-else-if="store.initError && !store.user"
        class="app-card p-6 text-center space-y-3"
      >
        <p class="text-sm font-bold text-[#18181B]">连接服务暂时失败</p>
        <p class="text-xs text-[#64748B]">{{ store.initError }}</p>
        <div class="flex items-center justify-center gap-2">
          <button
            type="button"
            class="px-4 py-2 rounded-full bg-[#F5F6F8] border border-[#E6E9EF] text-xs font-semibold text-[#18181B]"
            @click="store.initApp(true)"
          >
            重试连接
          </button>
          <button
            type="button"
            class="btn-amber px-4 py-2 rounded-full text-white text-xs font-semibold"
            @click="openAuthScreen('login')"
          >
            手动登录
          </button>
        </div>
      </div>

      <template v-else>
        <MobileRecordsView
          v-if="activeTab === 'records'"
          :search-query="searchQuery"
          @go-stats="activeTab = 'stats'"
          @edit-budget="openBudgetDetail"
          @open-invite="openInviteSheet"
          @open-ai="openRecordSheet('ai')"
          @open-record="openRecordSheet($event)"
          @select-tx="openTxDetail"
        />

        <MobileCalendarView
          v-else-if="activeTab === 'calendar'"
          @select-tx="openTxDetail"
          @add-for-date="openRecordForDate"
        />

        <MobileStatsView
          v-else-if="activeTab === 'stats'"
          @select-tx="openTxDetail"
          @open-ai="openRecordSheet('ai')"
        />

        <MobileProfileView
          v-else
          @open-ledger="ledgerSheetOpen = true"
          @open-invite="openInviteSheet"
          @open-subpage="openSubpageSheet"
          @open-auth="openAuthScreen"
        />
      </template>
    </main>

    <!-- 底部悬浮 4-Tab 液态玻璃 Dock + 琥珀渐变 + FAB -->
    <div
      v-if="!store.authRequired"
      class="fixed bottom-0 left-0 right-0 z-20 px-4 pointer-events-none"
      style="padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 14px)"
    >
      <div class="flex items-center gap-2.5 pointer-events-auto">
        <nav
          class="glass-dock flex-1 h-[56px] rounded-full p-1 flex items-center justify-between gap-0.5"
          aria-label="主导航"
        >
          <button
            type="button"
            class="flex-1 h-[46px] rounded-full flex flex-col items-center justify-center gap-0.5 transition-all duration-200"
            :class="activeTab === 'records' ? 'nav-tab-active' : 'text-[#94A3B8]'"
            @click="activeTab = 'records'"
          >
            <svg
              class="w-[18px] h-[18px] transition-colors"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="4" y="3.5" width="16" height="17" rx="3.5" />
              <path d="M8.5 8.5h7M8.5 12.5h7M8.5 16.5h4" />
            </svg>
            <span class="text-[10px] font-semibold tracking-tight">明细</span>
          </button>

          <button
            type="button"
            class="flex-1 h-[46px] rounded-full flex flex-col items-center justify-center gap-0.5 transition-all duration-200"
            :class="activeTab === 'calendar' ? 'nav-tab-active' : 'text-[#94A3B8]'"
            @click="activeTab = 'calendar'"
          >
            <svg
              class="w-[18px] h-[18px] transition-colors"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="3.5" y="5" width="17" height="15.5" rx="3.5" />
              <path d="M16 3v3.5M8 3v3.5M3.5 10h17" />
            </svg>
            <span class="text-[10px] font-semibold tracking-tight">日历</span>
          </button>

          <button
            type="button"
            class="flex-1 h-[46px] rounded-full flex flex-col items-center justify-center gap-0.5 transition-all duration-200"
            :class="activeTab === 'stats' ? 'nav-tab-active' : 'text-[#94A3B8]'"
            @click="activeTab = 'stats'"
          >
            <svg
              class="w-[18px] h-[18px] transition-colors"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M12 3.5v8.5h8.5" />
              <circle cx="12" cy="12" r="8.5" />
            </svg>
            <span class="text-[10px] font-semibold tracking-tight">统计</span>
          </button>

          <button
            type="button"
            class="flex-1 h-[46px] rounded-full flex flex-col items-center justify-center gap-0.5 transition-all duration-200"
            :class="activeTab === 'profile' ? 'nav-tab-active' : 'text-[#94A3B8]'"
            @click="activeTab = 'profile'"
          >
            <svg
              class="w-[18px] h-[18px] transition-colors"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="12" cy="8.5" r="3.5" />
              <path d="M5 19.5c0-3.2 3.1-5.5 7-5.5s7 2.3 7 5.5" />
            </svg>
            <span class="text-[10px] font-semibold tracking-tight">我的</span>
          </button>
        </nav>

        <!-- 主操作 FAB：记一笔 / AI 记账 -->
        <button
          type="button"
          title="记一笔 / AI 记账"
          aria-label="记一笔"
          class="glass-fab w-[56px] h-[56px] shrink-0 rounded-full text-white flex items-center justify-center active:scale-95 transition-all duration-200"
          @click="openRecordSheet('ai')"
        >
          <svg
            class="w-6 h-6"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.4"
            stroke-linecap="round"
          >
            <path d="M12 5v14M5 12h14" />
          </svg>
        </button>
      </div>
    </div>

    <!-- 全部抽屉与全屏管理页 -->
    <template v-if="!store.authRequired">
      <MobileRecordSheet
        v-model:open="recordSheetOpen"
        :initial-mode="recordInitialMode"
        :prefill-date="recordPrefillDate"
        @view-tx="openTxById"
      />

      <MobileLedgerSheet
        v-model:open="ledgerSheetOpen"
        @open-new-ledger="openSubpageSheet('new-ledger')"
      />

      <MobileInviteSheet
        v-model:open="inviteSheetOpen"
        :initial-view="inviteInitialView"
      />

      <MobileDetailSheet
        v-model:open="detailSheetOpen"
        :mode="detailMode"
        :transaction="selectedTransaction"
      />

      <MobileSubpageSheet
        v-model:open="subpageSheetOpen"
        :page="activeSubpage"
      />
    </template>

    <MobileAuthScreen
      :open="showAuthScreen"
      :required="store.authRequired"
      :initial-view="authInitialView"
      @update:open="(val) => { if (!val) authScreenOpen = false; }"
      @close="authScreenOpen = false"
      @success="authScreenOpen = false"
    />
  </div>
</template>
