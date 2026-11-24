<script setup lang="ts">
import { computed } from 'vue';
import { DEFAULT_MASCOT, MASCOT_FALLBACK, resolveAvatarUrl } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';
import type { PcViewId } from '@/pc/utils/adminAdapter';

defineProps<{
  activeView: PcViewId;
}>();

const emit = defineEmits<{
  'select-view': [view: PcViewId];
  'open-auth': [tab: 'login' | 'register' | 'forgot'];
}>();

const store = useAppStore();

interface NavItem {
  id: PcViewId;
  label: string;
  iconPath: string;
}

const USER_NAV_ITEMS: NavItem[] = [
  {
    id: 'dashboard',
    label: '数据概览',
    iconPath:
      '<rect x="3" y="3" width="7" height="9" rx="2"/><rect x="14" y="3" width="7" height="5" rx="2"/><rect x="14" y="12" width="7" height="9" rx="2"/><rect x="3" y="16" width="7" height="5" rx="2"/>',
  },
  {
    id: 'ledger',
    label: '流水明细',
    iconPath:
      '<rect x="4" y="3.5" width="16" height="17" rx="3"/><path d="M8 8.5h8M8 12.5h8M8 16.5h5"/>',
  },
  {
    id: 'calendar',
    label: '消费日历',
    iconPath:
      '<rect x="3.5" y="5" width="17" height="15.5" rx="3"/><path d="M16 3v3.5M8 3v3.5M3.5 10h17"/>',
  },
  {
    id: 'stats',
    label: '统计报表',
    iconPath: '<path d="M12 3.5v8.5h8.5"/><circle cx="12" cy="12" r="8.5"/>',
  },
  {
    id: 'finance',
    label: '资产与预算',
    iconPath:
      '<rect x="3.5" y="6" width="17" height="12" rx="3"/><path d="M3.5 10.5h17M7.5 14.5h3.5"/>',
  },
  {
    id: 'categories',
    label: '分类管理',
    iconPath:
      '<rect x="4" y="4" width="6.5" height="6.5" rx="1.8"/><rect x="13.5" y="4" width="6.5" height="6.5" rx="1.8"/><rect x="4" y="13.5" width="6.5" height="6.5" rx="1.8"/><path d="M16.75 13.5v6.5M13.5 16.75h6.5"/>',
  },
  {
    id: 'ledgers',
    label: '家庭账本',
    iconPath:
      '<circle cx="9" cy="8.5" r="3"/><path d="M3.5 18.5c0-2.8 2.5-4.8 5.5-4.8s5.5 2 5.5 4.8"/><path d="M17.5 9.5v5M15 12h5"/>',
  },
  {
    id: 'account',
    label: '账号与安全',
    iconPath:
      '<circle cx="12" cy="8.5" r="3.5"/><path d="M5 19.5c0-3.2 3.1-5.5 7-5.5s7 2.3 7 5.5"/>',
  },
];

const ADMIN_NAV_ITEMS: NavItem[] = [
  {
    id: 'admin-overview',
    label: '运营总览',
    iconPath: '<path d="M4 19h16M7 15v-4M12 15V7M17 15v-7"/>',
  },
  {
    id: 'admin-ai',
    label: 'AI 模型与用量',
    iconPath:
      '<path d="M12 3l1.8 5.4L19 10.2l-5.2 1.8L12 17.4l-1.8-5.4L5 10.2l5.2-1.8L12 3z"/>',
  },
  {
    id: 'admin-users',
    label: '用户与注册',
    iconPath:
      '<circle cx="9" cy="8.5" r="3"/><path d="M3.5 18.5c0-2.8 2.5-4.8 5.5-4.8s5.5 2 5.5 4.8"/><circle cx="17" cy="9.5" r="2.2"/><path d="M16 14.2c2.2.3 4 1.8 4 4.3"/>',
  },
  {
    id: 'admin-system',
    label: '系统与备份',
    iconPath:
      '<circle cx="12" cy="12" r="3"/><path d="M12 3v2.5M12 18.5V21M4.2 7.5l2.2 1.3M17.6 15.2l2.2 1.3M4.2 16.5l2.2-1.3M17.6 8.8l2.2-1.3"/>',
  },
];

const bookOptions = computed<DropdownOption[]>(() =>
  store.books.map((b) => ({
    value: b.id,
    label: b.name,
    dot: b.color || '#DE8A1E',
  }))
);

const userAvatar = computed(() => resolveAvatarUrl(store.user?.avatar));

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

async function handleSwitchBook(val: string | number) {
  const nextId = String(val);
  await store.switchBook(nextId);
  const book = store.books.find((b) => b.id === nextId);
  if (book) {
    store.showToast(`已切换至「${book.name}」`, 'success');
  }
}

function togglePreviewRole() {
  store.adminPreviewAsUser = !store.adminPreviewAsUser;
  store.showToast(
    store.isAdmin ? '已切至管理员视角（含中台）' : '已切至普通用户视角',
    'info'
  );
}
</script>

<template>
  <div class="flex flex-col h-full justify-between">
    <!-- 顶部品牌与导航列表 -->
    <div class="p-4 flex-1 overflow-y-auto thin-scroll space-y-5">
      <!-- 品牌 Logo -->
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2.5">
          <img
            :src="DEFAULT_MASCOT"
            alt="七猫记账"
            class="w-9 h-9 rounded-xl object-contain bg-white p-0.5 border border-[#EAECEF] shadow-xs"
            @error="onMascotError"
          />
          <div>
            <div class="flex items-center gap-1.5">
              <span class="text-sm font-bold text-[#18181B] tracking-tight">七猫记账</span>
              <span class="text-[9px] font-bold px-1.5 py-0.5 rounded bg-[#FFF6E8] text-[#DE8A1E]">
                Web
              </span>
            </div>
            <p class="text-[10px] text-[#94A3B8]">全功能 PC 工作台</p>
          </div>
        </div>
      </div>

      <!-- 账本切换选择器 -->
      <div>
        <CustomDropdown
          :model-value="store.activeBookId"
          :options="bookOptions"
          placeholder="选择账本"
          @change="handleSwitchBook"
        />
      </div>

      <!-- 个人与家庭记账分组 -->
      <div>
        <p class="px-3 text-[10px] font-semibold uppercase tracking-wider text-[#94A3B8] mb-1.5">
          个人与家庭
        </p>
        <nav class="space-y-0.5">
          <button
            v-for="item in USER_NAV_ITEMS"
            :key="item.id"
            type="button"
            class="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-semibold transition"
            :class="
              activeView === item.id
                ? 'nav-tab-active bg-[#FFF6E8] text-[#DE8A1E]'
                : 'text-[#475569] hover:bg-[#F4F5F8] hover:text-[#18181B]'
            "
            @click="emit('select-view', item.id)"
          >
            <svg
              class="w-4 h-4 shrink-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              v-html="item.iconPath"
            />
            <span class="truncate">{{ item.label }}</span>
          </button>
        </nav>
      </div>

      <!-- 轻量管理中台分组（仅管理员视角可见） -->
      <div v-if="store.isAdmin">
        <div class="px-3 flex items-center justify-between mb-1.5">
          <p class="text-[10px] font-semibold uppercase tracking-wider text-[#94A3B8]">
            轻量管理中台
          </p>
          <span class="text-[9px] font-semibold px-1.5 py-0.5 rounded bg-[#18181B] text-white">
            Admin
          </span>
        </div>
        <nav class="space-y-0.5">
          <button
            v-for="item in ADMIN_NAV_ITEMS"
            :key="item.id"
            type="button"
            class="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-semibold transition"
            :class="
              activeView === item.id
                ? 'nav-tab-active bg-[#FFF6E8] text-[#DE8A1E]'
                : 'text-[#475569] hover:bg-[#F4F5F8] hover:text-[#18181B]'
            "
            @click="emit('select-view', item.id)"
          >
            <svg
              class="w-4 h-4 shrink-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              v-html="item.iconPath"
            />
            <span class="truncate">{{ item.label }}</span>
          </button>
        </nav>
      </div>
    </div>

    <!-- 侧边栏底部：管理员视角开关与当前账号 -->
    <div class="p-3.5 border-t border-[#EAECEF] space-y-2.5 bg-[#FAFBFD] shrink-0">
      <button
        v-if="store.isRealAdmin"
        type="button"
        class="w-full px-3 py-2 rounded-xl bg-white border border-[#E6E9EF] hover:border-[#DE8A1E] flex items-center justify-between text-[11px] font-semibold transition"
        @click="togglePreviewRole"
      >
        <span class="text-[#64748B]">当前视角</span>
        <span class="text-[#DE8A1E]">
          {{ store.isAdmin ? '管理员（含中台）' : '普通用户预览' }}
        </span>
      </button>

      <div class="flex items-center justify-between gap-1.5">
        <button
          type="button"
          class="flex-1 min-w-0 flex items-center gap-2.5 px-1.5 py-1 rounded-xl hover:bg-white transition text-left"
          @click="emit('select-view', 'account')"
        >
          <img
            :src="userAvatar"
            :alt="store.user?.user_name || '用户头像'"
            class="w-8 h-8 rounded-full object-cover border border-[#EAECEF] shrink-0 bg-white"
          />
          <div class="min-w-0">
            <p class="text-xs font-bold text-[#18181B] truncate">
              {{ store.user?.user_name || '七猫用户' }}
            </p>
            <p class="text-[10px] text-[#94A3B8] truncate">
              {{ store.user?.email || '未绑定安全邮箱' }}
            </p>
          </div>
        </button>
        <button
          type="button"
          title="登录 / 注册 / 切换账号"
          class="shrink-0 px-2 py-1.5 rounded-lg bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-[10px] font-semibold text-[#475569] hover:text-[#DE8A1E] transition"
          @click="emit('open-auth', 'login')"
        >
          {{ store.hasAuthToken() ? '切换' : '登录' }}
        </button>
      </div>
    </div>
  </div>
</template>
