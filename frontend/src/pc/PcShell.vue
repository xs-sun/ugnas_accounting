<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useAppStore } from '@/stores/useAppStore';
import type { Transaction } from '@/types';
import type { PcViewId } from '@/pc/utils/adminAdapter';
import PcSidebarContent from '@/pc/components/PcSidebarContent.vue';
import PcDashboardView from '@/pc/views/PcDashboardView.vue';
import PcLedgerView from '@/pc/views/PcLedgerView.vue';
import PcCalendarView from '@/pc/views/PcCalendarView.vue';
import PcStatsView from '@/pc/views/PcStatsView.vue';
import PcFinanceView from '@/pc/views/PcFinanceView.vue';
import PcCategoriesView from '@/pc/views/PcCategoriesView.vue';
import PcLedgersView from '@/pc/views/PcLedgersView.vue';
import PcAccountView from '@/pc/views/PcAccountView.vue';
import PcAdminOverviewView from '@/pc/views/PcAdminOverviewView.vue';
import PcAdminAiView from '@/pc/views/PcAdminAiView.vue';
import PcAdminUsersView from '@/pc/views/PcAdminUsersView.vue';
import PcAdminSystemView from '@/pc/views/PcAdminSystemView.vue';
import PcRecordModal from '@/pc/modals/PcRecordModal.vue';
import PcAuthOverlay from '@/pc/modals/PcAuthOverlay.vue';
import PcImportExportModal from '@/pc/modals/PcImportExportModal.vue';

const store = useAppStore();

const VIEW_META: Record<PcViewId, { title: string; subtitle: string }> = {
  dashboard: { title: '数据概览', subtitle: '大屏收支趋势、预算水位与近期流水' },
  ledger: { title: '流水明细', subtitle: '支持多维筛选、批量整理与单笔编辑' },
  calendar: { title: '消费日历', subtitle: '按月俯瞰每日消费强弱与当日流水' },
  stats: { title: '统计报表', subtitle: '周期洞察、分类占比与单笔大额排行' },
  finance: { title: '资产与预算', subtitle: '统一管理资产账户、预算限额与定时规则' },
  categories: { title: '分类管理', subtitle: '自定义双色线框图标与一/二级分类体系' },
  ledgers: { title: '家庭账本', subtitle: '切换个人/共享账本，管理协作成员与口令邀请' },
  account: { title: '账号与安全', subtitle: '3D 小猫头像库、自定义上传与安全邮箱绑定' },
  'admin-overview': { title: '运营总览', subtitle: '仅聚合系统指标与活跃度 · 零私人账单访问' },
  'admin-ai': { title: 'AI 模型与用量', subtitle: '配置模型 API Key 与全站用户 Token 配额统计' },
  'admin-users': { title: '用户与注册', subtitle: '控制公开注册策略、邀请码与成员 AI 日限额' },
  'admin-system': { title: '系统与备份', subtitle: 'SMTP 发信邮箱、自动备份策略与 SQLite 热快照' },
};

const activeView = ref<PcViewId>('dashboard');
const tabletDrawerOpen = ref(false);

const recordModalOpen = ref(false);
const recordModalMode = ref<'ai' | 'manual'>('ai');
const editingTx = ref<Transaction | null>(null);
const recordDefaultDate = ref('');
const recordInitialPrompt = ref('');

const authOverlayOpen = ref(false);
const authInitialTab = ref<'login' | 'register' | 'forgot'>('login');
const importExportOpen = ref(false);

const showAuthOverlay = computed(
  () => authOverlayOpen.value || (store.initialized && store.authRequired)
);

const currentMeta = computed(() => VIEW_META[activeView.value] || VIEW_META.dashboard);

watch(
  () => store.isAdmin,
  (isAdminNow) => {
    if (!isAdminNow && activeView.value.startsWith('admin-')) {
      activeView.value = 'dashboard';
    }
  }
);

function handleSelectView(view: PcViewId) {
  if (view.startsWith('admin-') && !store.isAdmin) {
    activeView.value = 'dashboard';
  } else {
    activeView.value = view;
  }
  tabletDrawerOpen.value = false;
}

function openAuth(tab: 'login' | 'register' | 'forgot' = 'login') {
  authInitialTab.value = tab;
  authOverlayOpen.value = true;
  tabletDrawerOpen.value = false;
}

function openRecordModal(opts?: {
  mode?: 'ai' | 'manual';
  defaultDate?: string;
  prompt?: string;
}) {
  editingTx.value = null;
  recordModalMode.value = opts?.mode || 'ai';
  recordDefaultDate.value = opts?.defaultDate || '';
  recordInitialPrompt.value = opts?.prompt || '';
  recordModalOpen.value = true;
}

function openEditTransaction(tx: Transaction) {
  editingTx.value = tx;
  recordModalMode.value = 'manual';
  recordDefaultDate.value = '';
  recordInitialPrompt.value = '';
  recordModalOpen.value = true;
}

function closeRecordModal() {
  recordModalOpen.value = false;
  editingTx.value = null;
  recordInitialPrompt.value = '';
}

async function stepMonth(delta: number) {
  let nextMonth = store.currentMonth + delta;
  let nextYear = store.currentYear;
  if (nextMonth < 1) {
    nextMonth = 12;
    nextYear -= 1;
  } else if (nextMonth > 12) {
    nextMonth = 1;
    nextYear += 1;
  }
  await store.setCurrentMonth(nextYear, nextMonth);
}

function handleTogglePrivacy() {
  store.togglePrivacy();
  store.showToast(store.privacyHidden ? '已开启金额隐私脱敏' : '已显示账单金额', 'info');
}
</script>

<template>
  <div class="min-h-screen flex bg-[#F4F5F8] text-[#18181B]">
    <!-- 桌面端左侧固定 256px 导航栏 (>= 1024px) -->
    <aside
      class="hidden lg:flex w-[256px] shrink-0 bg-white border-r border-[#EAECEF] flex-col justify-between sticky top-0 h-screen select-none"
    >
      <PcSidebarContent
        :active-view="activeView"
        @select-view="handleSelectView"
        @open-auth="openAuth"
      />
    </aside>

    <!-- 平板端左侧滑出抽屉 (< 1024px) -->
    <div
      class="fixed inset-0 z-50 flex lg:hidden"
      :class="{ 'drawer-hidden': !tabletDrawerOpen }"
    >
      <div
        class="drawer-backdrop absolute inset-0 bg-[#0F172A]/35 backdrop-blur-[2px]"
        @click="tabletDrawerOpen = false"
      />
      <aside
        class="drawer-panel-left relative w-[264px] max-w-[84vw] h-full bg-white shadow-2xl flex flex-col justify-between"
      >
        <PcSidebarContent
          :active-view="activeView"
          @select-view="handleSelectView"
          @open-auth="openAuth"
        />
      </aside>
    </div>

    <!-- 右侧主工作区 -->
    <div class="flex-1 min-w-0 flex flex-col">
      <!-- 吸顶毛玻璃 Header -->
      <header
        class="sticky top-0 z-30 h-[62px] bg-white/85 backdrop-blur-xl border-b border-[#EAECEF] px-4 lg:px-7 flex items-center justify-between gap-3"
      >
        <div class="flex items-center gap-3 min-w-0">
          <button
            type="button"
            aria-label="打开导航菜单"
            class="lg:hidden w-9 h-9 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#18181B] flex items-center justify-center shrink-0 transition"
            @click="tabletDrawerOpen = true"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.1"
              stroke-linecap="round"
            >
              <path d="M4 7h16M4 12h16M4 17h12" />
            </svg>
          </button>

          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h1 class="text-[16px] font-bold text-[#18181B] tracking-tight truncate">
                {{ currentMeta.title }}
              </h1>
              <span
                v-if="store.activeBook"
                class="hidden xl:inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#F4F5F8] text-[#475569] shrink-0"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full"
                  :style="{ backgroundColor: store.activeBook.color || '#DE8A1E' }"
                />
                <span>{{ store.activeBook.name }}</span>
              </span>
            </div>
            <p class="hidden sm:block text-[11px] text-[#64748B] truncate mt-0.5">
              {{ currentMeta.subtitle }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <!-- 月份选择胶囊 -->
          <div
            class="hidden sm:inline-flex items-center h-9 px-1.5 rounded-xl bg-[#FFF6E8] border border-[#F2DFBE] text-xs font-semibold text-[#DE8A1E] gap-1"
          >
            <button
              type="button"
              title="上个月"
              class="w-6 h-6 rounded-lg hover:bg-white/80 flex items-center justify-center transition"
              @click="stepMonth(-1)"
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                <path d="M15 18l-6-6 6-6" />
              </svg>
            </button>
            <span class="px-1.5 tabular-nums whitespace-nowrap">
              {{ store.currentYear }}年{{ store.currentMonth }}月
            </span>
            <button
              type="button"
              title="下个月"
              class="w-6 h-6 rounded-lg hover:bg-white/80 flex items-center justify-center transition"
              @click="stepMonth(1)"
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                <path d="M9 18l6-6-6-6" />
              </svg>
            </button>
          </div>

          <!-- 隐私保护开关 -->
          <button
            type="button"
            :title="store.privacyHidden ? '显示金额' : '隐藏金额'"
            class="w-9 h-9 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] flex items-center justify-center transition"
            :class="store.privacyHidden ? 'text-[#DE8A1E]' : 'text-[#475569]'"
            @click="handleTogglePrivacy"
          >
            <svg
              v-if="!store.privacyHidden"
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
            >
              <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
            <svg
              v-else
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.9"
            >
              <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-6.5 0-10-8-10-8a18.45 18.45 0 0 1 5.06-5.94" />
              <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c6.5 0 10 8 10 8a18.5 18.5 0 0 1-2.16 3.19" />
              <line x1="1" y1="1" x2="23" y2="23" />
            </svg>
          </button>

          <!-- 导入导出快捷按钮 -->
          <button
            type="button"
            title="导入 / 导出 CSV 账单"
            class="h-9 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#475569] hover:text-[#18181B] text-xs font-semibold hidden md:inline-flex items-center gap-1.5 transition"
            @click="importExportOpen = true"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <span>导入导出</span>
          </button>

          <!-- 记一笔主按钮 -->
          <button
            type="button"
            class="btn-amber h-9 px-4 rounded-xl text-white text-xs font-semibold flex items-center gap-1.5 transition"
            @click="openRecordModal({ mode: 'ai' })"
          >
            <span>✦</span>
            <span>记一笔</span>
          </button>
        </div>
      </header>

      <!-- 主内容区（动态渲染 12 个视图） -->
      <main class="flex-1 p-4 lg:p-7 max-w-[1340px] w-full mx-auto">
        <div
          v-if="!store.initialized && store.loading"
          class="pc-card p-10 text-center text-xs text-[#64748B]"
        >
          正在校验登录状态与同步账本数据…
        </div>
        <div
          v-else-if="store.authRequired"
          class="pc-card p-10 text-center space-y-3"
        >
          <p class="text-base font-bold text-[#18181B]">当前处于未登录状态</p>
          <p class="text-xs text-[#64748B]">未检测到 NAS 网关登录信息，请登录或注册独立账号继续使用</p>
          <button
            type="button"
            class="btn-amber px-5 py-2.5 rounded-xl text-white text-xs font-semibold"
            @click="openAuth('login')"
          >
            前往登录 / 注册
          </button>
        </div>
        <div
          v-else-if="store.initError && !store.user"
          class="pc-card p-10 text-center space-y-3"
        >
          <p class="text-base font-bold text-[#18181B]">连接服务暂时失败</p>
          <p class="text-xs text-[#64748B]">{{ store.initError }}</p>
          <div class="flex items-center justify-center gap-2.5">
            <button
              type="button"
              class="px-4 py-2 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#18181B] transition"
              @click="store.initApp(true)"
            >
              重试连接
            </button>
            <button
              type="button"
              class="btn-amber px-4 py-2 rounded-xl text-white text-xs font-semibold"
              @click="openAuth('login')"
            >
              手动登录
            </button>
          </div>
        </div>
        <template v-else>
          <PcDashboardView
            v-if="activeView === 'dashboard'"
            @navigate="handleSelectView"
            @edit-tx="openEditTransaction"
            @open-record="openRecordModal"
            @open-import-export="importExportOpen = true"
          />
          <PcLedgerView
            v-else-if="activeView === 'ledger'"
            @edit-tx="openEditTransaction"
            @open-import-export="importExportOpen = true"
            @open-record="openRecordModal"
          />
          <PcCalendarView
            v-else-if="activeView === 'calendar'"
            @edit-tx="openEditTransaction"
            @open-record="openRecordModal"
          />
          <PcStatsView
            v-else-if="activeView === 'stats'"
            @edit-tx="openEditTransaction"
            @open-record="openRecordModal"
          />
          <PcFinanceView v-else-if="activeView === 'finance'" />
          <PcCategoriesView v-else-if="activeView === 'categories'" />
          <PcLedgersView v-else-if="activeView === 'ledgers'" />
          <PcAccountView
            v-else-if="activeView === 'account'"
            @open-auth="openAuth"
          />
          <PcAdminOverviewView
            v-else-if="activeView === 'admin-overview'"
            @navigate="handleSelectView"
          />
          <PcAdminAiView v-else-if="activeView === 'admin-ai'" />
          <PcAdminUsersView v-else-if="activeView === 'admin-users'" />
          <PcAdminSystemView v-else-if="activeView === 'admin-system'" />
        </template>
      </main>
    </div>

    <!-- 全局弹窗与浮层 -->
    <PcRecordModal
      :open="recordModalOpen"
      :initial-mode="recordModalMode"
      :editing-tx="editingTx"
      :default-date="recordDefaultDate"
      :initial-ai-prompt="recordInitialPrompt"
      @close="closeRecordModal"
    />

    <PcAuthOverlay
      :open="showAuthOverlay"
      :required="store.authRequired"
      :initial-view="authInitialTab"
      @close="authOverlayOpen = false"
      @success="authOverlayOpen = false"
    />

    <PcImportExportModal
      :open="importExportOpen"
      @close="importExportOpen = false"
    />
  </div>
</template>
