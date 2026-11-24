<script setup lang="ts">
import { ref, computed } from 'vue';
import { useAppStore } from '@/stores/useAppStore';
import { DEFAULT_MASCOT, MASCOT_FALLBACK, resolveAvatarUrl } from '@/constants/avatars';
import { formatMoney, getWeekdayLabel } from '@/utils/format';
import type { BookMember, Transaction } from '@/types';
import type { InviteSheetView, RecordInitialMode } from '../types';
import BudgetProgressBar from '@/components/shared/BudgetProgressBar.vue';
import TransactionRow from '@/components/shared/TransactionRow.vue';

const props = withDefaults(
  defineProps<{
    searchQuery?: string;
  }>(),
  {
    searchQuery: '',
  }
);

const emit = defineEmits<{
  'go-stats': [];
  'edit-budget': [];
  'open-invite': [view?: InviteSheetView];
  'open-ai': [];
  'open-record': [mode: RecordInitialMode];
  'select-tx': [tx: Transaction];
}>();

const store = useAppStore();
const selectedCategoryId = ref<string>('all');

const monthPrefix = computed(
  () => `${store.currentYear}-${String(store.currentMonth).padStart(2, '0')}-`
);

// 当月交易汇总（用于 Hero 卡片统计）
const currentMonthTxs = computed(() =>
  store.transactions.filter((tx) => tx.date && tx.date.startsWith(monthPrefix.value))
);

const monthExpense = computed(() =>
  currentMonthTxs.value
    .filter((tx) => tx.type === 'expense')
    .reduce((sum, tx) => sum + Number(tx.amount || 0), 0)
);

const monthIncome = computed(() =>
  currentMonthTxs.value
    .filter((tx) => tx.type === 'income')
    .reduce((sum, tx) => sum + Number(tx.amount || 0), 0)
);

const monthBalance = computed(() => monthIncome.value - monthExpense.value);

// 预算脉搏计算（优先读取后端聚合数据，缺失时前端兜底）
const budgetLimit = computed(() => {
  if (store.budgets && store.budgets.total_budget >= 0) {
    return store.budgets.total_budget;
  }
  return store.activeBook?.budget || 0;
});

const budgetSpent = computed(() => {
  if (store.budgets && typeof store.budgets.total_spent === 'number') {
    return store.budgets.total_spent;
  }
  return currentMonthTxs.value
    .filter((tx) => tx.type === 'expense' && tx.include_budget !== false)
    .reduce((sum, tx) => sum + Number(tx.amount || 0), 0);
});

const budgetPct = computed(() => {
  if (store.budgets && typeof store.budgets.percentage === 'number' && budgetLimit.value > 0) {
    return Math.round(store.budgets.percentage);
  }
  if (budgetLimit.value <= 0) return 0;
  return Math.round((budgetSpent.value / budgetLimit.value) * 100);
});

const remainingDays = computed(() => {
  if (store.budgets && store.budgets.remaining_days > 0) {
    return store.budgets.remaining_days;
  }
  const now = new Date();
  const daysInMonth = new Date(store.currentYear, store.currentMonth, 0).getDate();
  if (
    store.currentYear === now.getFullYear() &&
    store.currentMonth === now.getMonth() + 1
  ) {
    return Math.max(1, daysInMonth - now.getDate() + 1);
  }
  return daysInMonth;
});

const budgetRemaining = computed(() => {
  if (store.budgets && typeof store.budgets.remaining_budget === 'number') {
    return store.budgets.remaining_budget;
  }
  return Math.max(0, budgetLimit.value - budgetSpent.value);
});

const dailySafeAllowance = computed(() => {
  if (store.budgets && typeof store.budgets.daily_allowance === 'number') {
    return store.budgets.daily_allowance;
  }
  if (budgetLimit.value <= 0 || budgetRemaining.value <= 0) return 0;
  return Math.round(budgetRemaining.value / Math.max(1, remainingDays.value));
});

const budgetStatusBadge = computed(() => {
  if (budgetLimit.value <= 0) {
    return { text: '未设预算', cls: 'bg-[#F1F5F9] text-[#64748B]' };
  }
  if (budgetSpent.value > budgetLimit.value) {
    return { text: '预算超支', cls: 'bg-[#FDF0EE] text-[#D6553E]' };
  }
  if (budgetPct.value >= 85) {
    return { text: '临近上限', cls: 'bg-[#FEF6E7] text-[#D9822B]' };
  }
  return { text: '预算正常', cls: 'bg-[#FFF6E8] text-[#DE8A1E]' };
});

const budgetPctColorClass = computed(() => {
  if (budgetLimit.value > 0 && (budgetSpent.value > budgetLimit.value || budgetPct.value >= 85)) {
    return 'text-[#D6553E]';
  }
  if (budgetPct.value >= 65) return 'text-[#D9822B]';
  return 'text-[#DE8A1E]';
});

// 账本成员栈
const bookMembers = computed<BookMember[]>(() => {
  const list = store.activeBook?.members || [];
  if (list.length > 0) return list;
  if (store.user) {
    return [
      {
        book_id: store.activeBookId,
        user_id: store.user.user_id,
        user_name: store.user.user_name || '我',
        role: 'owner',
        avatar: store.user.avatar,
        color: store.activeBook?.color || '#DE8A1E',
        is_virtual: false,
        status: 'active',
        is_me: true,
        created_at: '',
      },
    ];
  }
  return [];
});

// 快捷分类筛选胶囊
const filterPills = computed(() => {
  const pills: Array<{ id: string; name: string }> = [{ id: 'all', name: '全部' }];
  const seen = new Set<string>(['all']);
  for (const cat of store.expenseCategories.slice(0, 6)) {
    if (!seen.has(cat.id)) {
      seen.add(cat.id);
      pills.push({ id: cat.id, name: cat.name });
    }
  }
  for (const cat of store.incomeCategories.slice(0, 2)) {
    if (!seen.has(cat.id)) {
      seen.add(cat.id);
      pills.push({ id: cat.id, name: cat.name });
    }
  }
  return pills;
});

// 联合过滤（分类筛选 + 顶部搜索关键词）
const filteredTransactions = computed(() => {
  let list = [...store.transactions];

  if (selectedCategoryId.value !== 'all') {
    list = list.filter((tx) => tx.category_id === selectedCategoryId.value);
  }

  const q = props.searchQuery.trim().toLowerCase();
  if (q) {
    list = list.filter((tx) => {
      const fields = [
        tx.title,
        tx.note,
        tx.category_name,
        tx.sub_category,
        tx.wallet_name,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return fields.includes(q);
    });
  }

  return list.sort((a, b) => {
    const keyA = `${a.date || ''} ${a.tx_time || ''} ${a.created_at || ''}`;
    const keyB = `${b.date || ''} ${b.tx_time || ''} ${b.created_at || ''}`;
    return keyB.localeCompare(keyA);
  });
});

interface DayGroup {
  date: string;
  label: string;
  expense: number;
  income: number;
  items: Transaction[];
}

const groupedTransactions = computed<DayGroup[]>(() => {
  const map = new Map<string, Transaction[]>();
  for (const tx of filteredTransactions.value) {
    const d = (tx.date || '').slice(0, 10) || '未知日期';
    if (!map.has(d)) map.set(d, []);
    map.get(d)!.push(tx);
  }

  const sortedDates = Array.from(map.keys()).sort((a, b) => b.localeCompare(a));
  return sortedDates.map((date) => {
    const items = map.get(date)!;
    let expense = 0;
    let income = 0;
    for (const item of items) {
      if (item.type === 'expense') expense += Number(item.amount || 0);
      else if (item.type === 'income') income += Number(item.amount || 0);
    }
    return {
      date,
      label: getWeekdayLabel(date),
      expense,
      income,
      items,
    };
  });
});

function hasAvatarImage(member: BookMember): boolean {
  return Boolean(member.is_me || (member.avatar && member.avatar.trim()));
}

function getMemberAvatar(member: BookMember): string {
  if (member.is_me) {
    return resolveAvatarUrl(store.user?.avatar || member.avatar);
  }
  return resolveAvatarUrl(member.avatar);
}

function getMemberInitial(member: BookMember): string {
  const name = (member.user_name || '成员').trim();
  return name.slice(0, 1).toUpperCase();
}

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}
</script>

<template>
  <div class="space-y-3.5">
    <!-- Hero 月度预算脉搏卡片 -->
    <section class="app-card p-4 relative overflow-hidden">
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-1.5">
          <span class="text-xs font-semibold text-[#18181B]">
            {{ store.currentYear }}年{{ store.currentMonth }}月
          </span>
          <span
            class="text-[10px] font-medium px-2 py-0.5 rounded-full"
            :class="budgetStatusBadge.cls"
          >
            {{ budgetStatusBadge.text }}
          </span>
        </div>
        <button
          type="button"
          class="text-[11px] font-medium text-[#DE8A1E] flex items-center gap-0.5 hover:underline"
          @click="emit('go-stats')"
        >
          <span>月度报表</span>
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
            <path d="M9 18l6-6-6-6" />
          </svg>
        </button>
      </div>

      <div class="flex items-end justify-between mb-3.5 gap-2">
        <div class="min-w-0">
          <p class="text-[11px] text-[#64748B] mb-0.5">本月总支出</p>
          <p class="text-[28px] leading-none font-bold tracking-tight tabular-nums text-[#18181B] truncate">
            {{ formatMoney(monthExpense, { privacy: store.privacyHidden }) }}
          </p>
        </div>
        <div class="text-right space-y-0.5 shrink-0">
          <p class="text-[11px] text-[#64748B] tabular-nums">
            收入
            <span class="font-semibold text-[#3E8E63]">
              {{ formatMoney(monthIncome, { privacy: store.privacyHidden }) }}
            </span>
          </p>
          <p class="text-[11px] text-[#64748B] tabular-nums">
            结余
            <span
              class="font-semibold"
              :class="monthBalance < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
            >
              {{ formatMoney(monthBalance, { privacy: store.privacyHidden }) }}
            </span>
          </p>
        </div>
      </div>

      <!-- 预算进度条与日均可用 -->
      <div class="rounded-2xl bg-[#F7F8FA] border border-[#ECEEF2] p-3">
        <div class="flex items-center justify-between text-[11px] mb-1.5">
          <button
            type="button"
            class="font-medium text-[#475569] flex items-center gap-1 hover:text-[#DE8A1E] transition"
            @click="emit('edit-budget')"
          >
            <span>
              月预算
              {{
                budgetLimit > 0
                  ? formatMoney(budgetLimit, { privacy: store.privacyHidden, decimals: 0 })
                  : '未设置'
              }}
            </span>
            <svg class="w-3 h-3 text-[#94A3B8]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 20h9" />
              <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" />
            </svg>
          </button>
          <span class="font-semibold tabular-nums" :class="budgetPctColorClass">
            {{ budgetLimit > 0 ? `已用 ${budgetPct}%` : '点击设定' }}
          </span>
        </div>

        <BudgetProgressBar
          :percentage="budgetPct"
          :spent="budgetSpent"
          :limit="budgetLimit"
          :privacy="store.privacyHidden"
          height-class="h-2"
        />

        <div class="flex items-center justify-between text-[11px] mt-2">
          <span class="text-[#64748B]">
            <template v-if="budgetLimit > 0 && budgetSpent > budgetLimit">
              本月已超支 · 建议控制开销
            </template>
            <template v-else-if="budgetLimit > 0">
              剩余 {{ remainingDays }} 天 · 建议日均可用
            </template>
            <template v-else>
              设置月度预算，实时追踪花销节奏
            </template>
          </span>
          <span
            v-if="budgetLimit > 0 && budgetSpent > budgetLimit"
            class="font-bold text-[#D6553E] tabular-nums"
          >
            -{{ formatMoney(budgetSpent - budgetLimit, { privacy: store.privacyHidden }) }}
          </span>
          <span v-else-if="budgetLimit > 0" class="font-bold text-[#DE8A1E] tabular-nums">
            {{ formatMoney(dailySafeAllowance, { privacy: store.privacyHidden }) }} / 天
          </span>
          <button
            v-else
            type="button"
            class="font-semibold text-[#DE8A1E] hover:underline"
            @click="emit('edit-budget')"
          >
            立即设置
          </button>
        </div>
      </div>

      <!-- 共享账本成员协作条 -->
      <div class="mt-3 pt-3 border-t border-[#F1F3F6] flex items-center justify-between gap-2">
        <button
          type="button"
          class="flex items-center gap-2 hover:opacity-85 transition min-w-0"
          @click="emit('open-invite', 'members')"
        >
          <div class="flex -space-x-1.5 shrink-0">
            <template v-for="m in bookMembers.slice(0, 5)" :key="m.user_id">
              <img
                v-if="hasAvatarImage(m)"
                :src="getMemberAvatar(m)"
                :alt="m.user_name"
                :title="m.user_name"
                class="w-6 h-6 rounded-full object-cover ring-2 ring-white bg-white"
              />
              <span
                v-else
                class="w-6 h-6 rounded-full text-[10px] font-bold text-white flex items-center justify-center ring-2 ring-white"
                :style="{ backgroundColor: m.color || '#DE8A1E' }"
                :title="m.user_name"
              >
                {{ getMemberInitial(m) }}
              </span>
            </template>
          </div>
          <span class="text-[11px] font-medium text-[#64748B] truncate">
            {{ bookMembers.length }} 位成员参与
          </span>
        </button>

        <button
          type="button"
          class="text-[11px] font-semibold px-3 py-1 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition flex items-center gap-1 shrink-0"
          @click="emit('open-invite', 'invite')"
        >
          <span>+</span>
          <span>邀请成员</span>
        </button>
      </div>
    </section>

    <!-- 七猫 AI 快捷记账入口条 -->
    <button
      type="button"
      class="w-full app-card px-3.5 py-2.5 flex items-center justify-between gap-2 hover:border-[#DE8A1E]/40 transition text-left"
      @click="emit('open-ai')"
    >
      <div class="flex items-center gap-2.5 min-w-0">
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫助手"
          class="w-7 h-7 rounded-full object-contain bg-white p-0.5 border border-[#F4E4C8] shrink-0"
          @error="onMascotError"
        />
        <div class="min-w-0">
          <p class="text-xs font-semibold text-[#18181B] truncate">跟小七聊一句记账或查账</p>
          <p class="text-[10px] text-[#94A3B8] truncate">
            试试说 “午饭外卖 32 元” 或 “本月餐饮占比”
          </p>
        </div>
      </div>
      <span class="text-[11px] font-semibold px-2.5 py-1 rounded-full bg-[#FFF6E8] text-[#DE8A1E] shrink-0">
        ✦ AI
      </span>
    </button>

    <!-- 分类快捷筛选胶囊条 -->
    <div class="flex items-center gap-1.5 overflow-x-auto phone-scroll pt-0.5">
      <button
        v-for="pill in filterPills"
        :key="pill.id"
        type="button"
        class="shrink-0 text-xs font-medium px-3 py-1.5 rounded-full transition"
        :class="
          selectedCategoryId === pill.id
            ? 'bg-[#18181B] text-white shadow-xs'
            : 'bg-white border border-[#ECEEF2] text-[#64748B] hover:text-[#18181B]'
        "
        @click="selectedCategoryId = pill.id"
      >
        {{ pill.name }}
      </button>
    </div>

    <!-- 按日期倒序分组的账单流水列表 -->
    <div v-if="groupedTransactions.length === 0" class="app-card p-8 text-center">
      <img
        :src="DEFAULT_MASCOT"
        alt="空状态"
        class="w-14 h-14 rounded-full mx-auto mb-3 object-contain bg-white p-1 border border-[#ECEEF2] opacity-90"
        @error="onMascotError"
      />
      <p class="text-sm font-semibold text-[#18181B]">
        {{ searchQuery.trim() || selectedCategoryId !== 'all' ? '没有找到匹配的账单' : '还没有记账流水' }}
      </p>
      <p class="text-xs text-[#94A3B8] mt-1">
        {{ searchQuery.trim() || selectedCategoryId !== 'all' ? '换个关键词或分类筛选条件试试' : '点右下角「+」或者让小七帮你记一笔' }}
      </p>
      <div class="mt-4 flex items-center justify-center gap-2">
        <button
          type="button"
          class="text-xs font-semibold px-3.5 py-2 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition"
          @click="emit('open-record', 'ai')"
        >
          ✦ AI 聊着记
        </button>
        <button
          type="button"
          class="text-xs font-semibold px-3.5 py-2 rounded-full bg-[#18181B] text-white hover:bg-[#27272A] transition"
          @click="emit('open-record', 'expense')"
        >
          手动记一笔
        </button>
      </div>
    </div>

    <template v-else>
      <section
        v-for="group in groupedTransactions"
        :key="group.date"
        class="app-card overflow-hidden"
      >
        <div
          class="px-4 py-2.5 bg-[#F8F9FB] border-b border-[#ECEEF2] flex items-center justify-between text-[11px]"
        >
          <span class="font-semibold text-[#27272A]">{{ group.label }}</span>
          <span class="text-[#94A3B8] tabular-nums">
            <template v-if="group.expense > 0">
              支 {{ formatMoney(group.expense, { privacy: store.privacyHidden }) }}
            </template>
            <template v-if="group.expense > 0 && group.income > 0"> · </template>
            <template v-if="group.income > 0">
              收 {{ formatMoney(group.income, { privacy: store.privacyHidden }) }}
            </template>
          </span>
        </div>

        <div class="divide-y divide-[#F1F3F6]">
          <TransactionRow
            v-for="tx in group.items"
            :key="tx.id"
            :tx="tx"
            :privacy="store.privacyHidden"
            @click="emit('select-tx', $event)"
          />
        </div>
      </section>
    </template>
  </div>
</template>
