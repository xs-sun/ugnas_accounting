<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { api } from '@/api/client';
import { resolveAvatarUrl } from '@/constants/avatars';
import { formatTokens } from '@/utils/format';
import {
  normalizeAdminOverview,
  normalizeAiConfig,
  normalizeRegConfig,
  normalizeSystemConfig,
  type PcViewId,
  type NormalizedAdminOverview,
  type NormalizedAiConfig,
  type NormalizedRegConfig,
  type NormalizedSystemConfig,
} from '@/pc/utils/adminAdapter';

const emit = defineEmits<{
  navigate: [view: PcViewId];
}>();

const loading = ref(true);
const errorMsg = ref('');
const overview = ref<NormalizedAdminOverview | null>(null);
const aiCfg = ref<NormalizedAiConfig | null>(null);
const regCfg = ref<NormalizedRegConfig | null>(null);
const sysCfg = ref<NormalizedSystemConfig | null>(null);

async function loadOverview() {
  loading.value = true;
  errorMsg.value = '';
  try {
    const [ovRes, aiRes, regRes, sysRes] = await Promise.allSettled([
      api.getAdminOverview(),
      api.getAdminAiConfig(),
      api.getAdminRegConfig(),
      api.getAdminSystemConfig(),
    ]);

    if (ovRes.status === 'fulfilled') {
      overview.value = normalizeAdminOverview(ovRes.value);
    } else {
      errorMsg.value = ovRes.reason?.message || '获取运营总览数据失败';
    }
    if (aiRes.status === 'fulfilled') aiCfg.value = normalizeAiConfig(aiRes.value);
    if (regRes.status === 'fulfilled') regCfg.value = normalizeRegConfig(regRes.value);
    if (sysRes.status === 'fulfilled') sysCfg.value = normalizeSystemConfig(sysRes.value);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadOverview();
});

const weeklyActiveRate = computed<number>(() => {
  if (!overview.value || overview.value.totalUsers <= 0) return 0;
  return Math.min(
    100,
    Math.round((overview.value.weeklyActiveUsers / overview.value.totalUsers) * 100)
  );
});

const maxActivityCount = computed<number>(() => {
  const list = overview.value?.dailyActivity || [];
  const max = Math.max(0, ...list.map((d) => d.txCount || 0));
  return max > 0 ? max : 10;
});

function getBarHeightPct(val: number): number {
  if (!val || val <= 0) return 6;
  return Math.max(10, Math.round((val / maxActivityCount.value) * 100));
}
</script>

<template>
  <div class="space-y-5">
    <!-- 隐私边界宣言横幅 -->
    <section
      class="rounded-2xl px-4 py-3 bg-[#FFF6E8] border border-[#F4DFBD] flex flex-wrap items-center justify-between gap-2 text-xs text-[#5B3A13]"
    >
      <span>
        <strong>隐私保护原则：</strong>
        管理中台仅聚合系统运行状态、匿名活跃度、累计记账笔数与 AI Token 配额统计，不访问也不展示任何用户的私人账单明细。
      </span>
      <div class="flex items-center gap-3 shrink-0">
        <span class="font-semibold text-[#DE8A1E]">NAS 实例运行正常</span>
        <button
          type="button"
          class="px-2.5 py-1 rounded-lg bg-white/80 hover:bg-white text-[11px] font-semibold text-[#475569] transition"
          @click="loadOverview"
        >
          {{ loading ? '刷新中…' : '刷新数据' }}
        </button>
      </div>
    </section>

    <div
      v-if="errorMsg"
      class="pc-card p-5 border-[#F5C4BC] bg-[#FDF0EE] text-xs text-[#D6553E] font-medium"
    >
      {{ errorMsg }}
    </div>

    <!-- 4 列系统指标卡 -->
    <section class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">注册用户总数</p>
        <p class="text-2xl font-bold text-[#18181B] tabular-nums mt-1.5">
          {{ overview?.totalUsers ?? 0 }} 人
        </p>
        <p class="text-[11px] text-[#3E8E63] mt-2 font-medium">
          启用中 {{ overview?.activeUsers ?? 0 }} 人
        </p>
      </div>

      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">近 7 日活跃用户</p>
        <p class="text-2xl font-bold text-[#18181B] tabular-nums mt-1.5">
          {{ overview?.weeklyActiveUsers ?? 0 }} / {{ overview?.totalUsers ?? 0 }}
        </p>
        <p class="text-[11px] text-[#DE8A1E] mt-2 font-medium">
          周活跃率 {{ weeklyActiveRate }}%
        </p>
      </div>

      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">账本与累计流水</p>
        <p class="text-2xl font-bold text-[#18181B] tabular-nums mt-1.5">
          {{ (overview?.totalTransactions ?? 0).toLocaleString() }} 笔
        </p>
        <p class="text-[11px] text-[#64748B] mt-2">
          全站共 {{ overview?.totalBooks ?? 0 }} 本共享/个人账本
        </p>
      </div>

      <div class="pc-card p-5">
        <p class="text-xs text-[#64748B]">全站 AI Token 消耗</p>
        <p class="text-2xl font-bold text-[#DE8A1E] tabular-nums mt-1.5">
          {{ formatTokens(overview?.totalTokens ?? 0) }}
        </p>
        <p class="text-[11px] text-[#64748B] mt-2 tabular-nums">
          今日 {{ overview?.todayAiCalls ?? 0 }} 次 · 累计 {{ overview?.totalAiCalls ?? 0 }} 次
        </p>
      </div>
    </section>

    <!-- 活跃度趋势与系统配置健康摘要 -->
    <section class="grid grid-cols-1 lg:grid-cols-12 gap-5">
      <!-- 近 7 日全站记账活跃度 -->
      <div class="pc-card p-5 lg:col-span-5 flex flex-col justify-between">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">近 7 日全站记账活跃度</h2>
          <p class="text-[11px] text-[#94A3B8] mt-0.5">每日匿名汇总记账笔数</p>
        </div>

        <div
          v-if="!overview?.dailyActivity?.length"
          class="h-44 flex items-center justify-center text-xs text-[#94A3B8]"
        >
          暂无近期活跃记录
        </div>
        <div v-else class="h-44 flex items-end justify-between gap-2 pt-4">
          <div
            v-for="item in overview.dailyActivity"
            :key="item.date || item.label"
            class="flex-1 h-full flex flex-col items-center justify-end gap-1.5"
          >
            <span class="text-[10px] text-[#64748B] tabular-nums">
              {{ item.txCount > 0 ? item.txCount : '' }}
            </span>
            <div
              class="w-full max-w-[28px] rounded-t-lg transition-all"
              :class="item.txCount > 0 ? 'bg-[#DE8A1E]/85' : 'bg-[#F1F3F6]'"
              :style="{ height: `${getBarHeightPct(item.txCount)}%` }"
            />
            <span class="text-[10px] text-[#94A3B8] tabular-nums">
              {{ item.label || item.date.slice(5) }}
            </span>
          </div>
        </div>
      </div>

      <!-- AI 服务商、注册策略与 SMTP/备份健康摘要 -->
      <div class="pc-card p-5 lg:col-span-7 grid grid-cols-1 sm:grid-cols-3 gap-3.5">
        <div class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex flex-col justify-between space-y-3">
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-[#18181B]">AI 服务状态</span>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                :class="
                  aiCfg?.enabled ?? true
                    ? 'bg-[#ECF6F0] text-[#3E8E63]'
                    : 'bg-[#FDF0EE] text-[#D6553E]'
                "
              >
                {{ (aiCfg?.enabled ?? true) ? '已启用' : '已停用' }}
              </span>
            </div>
            <p class="text-xs font-semibold text-[#DE8A1E] truncate">
              {{ aiCfg?.provider || overview?.aiProvider || 'DeepSeek' }}
            </p>
            <p class="text-[11px] text-[#64748B] truncate font-mono">
              {{ aiCfg?.model || overview?.aiModel || 'deepseek-chat' }}
            </p>
            <p class="text-[10px] text-[#94A3B8]">
              默认日限额 {{ aiCfg?.dailyLimit ?? 30 }} 次/人
            </p>
          </div>
          <button
            type="button"
            class="w-full py-1.5 rounded-xl bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-[11px] font-semibold text-[#475569] hover:text-[#DE8A1E] transition"
            @click="emit('navigate', 'admin-ai')"
          >
            配置模型与用量 →
          </button>
        </div>

        <div class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex flex-col justify-between space-y-3">
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-[#18181B]">注册策略</span>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                :class="
                  regCfg?.allowRegister ?? true
                    ? 'bg-[#ECF6F0] text-[#3E8E63]'
                    : 'bg-[#FEF6E7] text-[#D9822B]'
                "
              >
                {{ (regCfg?.allowRegister ?? true) ? '公开注册' : '仅邀请' }}
              </span>
            </div>
            <p class="text-[11px] text-[#64748B]">
              邀请码验证：{{ regCfg?.requireInviteCode ? '已开启' : '未开启' }}
            </p>
            <p class="text-[11px] text-[#64748B]">
              邮箱验证：{{ regCfg?.requireEmailVerify ? '强制验证' : '选填' }}
            </p>
            <p class="text-[10px] text-[#94A3B8]">
              新用户默认限额 {{ regCfg?.defaultDailyAiLimit ?? 30 }} 次/天
            </p>
          </div>
          <button
            type="button"
            class="w-full py-1.5 rounded-xl bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-[11px] font-semibold text-[#475569] hover:text-[#DE8A1E] transition"
            @click="emit('navigate', 'admin-users')"
          >
            管理用户与策略 →
          </button>
        </div>

        <div class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex flex-col justify-between space-y-3">
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-[#18181B]">SMTP 与备份</span>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                :class="
                  sysCfg?.autoBackupEnabled
                    ? 'bg-[#ECF6F0] text-[#3E8E63]'
                    : 'bg-[#F4F5F8] text-[#64748B]'
                "
              >
                {{ sysCfg?.autoBackupEnabled ? '自动备份中' : '手动备份' }}
              </span>
            </div>
            <p class="text-[11px] text-[#64748B] truncate">
              SMTP：{{ sysCfg?.smtpHost || '未配置服务器' }}
            </p>
            <p class="text-[11px] text-[#64748B]">
              备份周期：{{ sysCfg?.autoBackupCycle || '每天 03:00' }}
            </p>
            <p class="text-[10px] text-[#94A3B8]">
              保留最近 {{ sysCfg?.autoBackupRetain ?? 14 }} 份热快照
            </p>
          </div>
          <button
            type="button"
            class="w-full py-1.5 rounded-xl bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-[11px] font-semibold text-[#475569] hover:text-[#DE8A1E] transition"
            @click="emit('navigate', 'admin-system')"
          >
            系统与快照管理 →
          </button>
        </div>
      </div>
    </section>

    <!-- 用户宏观计量总览表（仅聚合计数与 Token，不含任何私人账单） -->
    <section class="pc-card overflow-hidden">
      <div class="px-5 py-3.5 border-b border-[#F1F3F6] flex items-center justify-between">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">用户宏观计量总览</h2>
          <p class="text-[11px] text-[#94A3B8]">仅展示各用户的账本数量、总笔数与 AI 配额消耗</p>
        </div>
        <button
          type="button"
          class="text-xs font-semibold text-[#DE8A1E] hover:underline"
          @click="emit('navigate', 'admin-users')"
        >
          成员与限额管理 →
        </button>
      </div>

      <div v-if="!overview?.users?.length" class="py-10 text-center text-xs text-[#94A3B8]">
        {{ loading ? '正在加载统计数据…' : '暂无用户计量数据' }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
              <th class="py-2.5 px-5 font-semibold">用户</th>
              <th class="py-2.5 px-3 font-semibold text-right">账本数</th>
              <th class="py-2.5 px-3 font-semibold text-right">累计记账</th>
              <th class="py-2.5 px-3 font-semibold text-right">累计 Token</th>
              <th class="py-2.5 px-5 font-semibold text-right">最近活跃</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#F1F3F6]">
            <tr v-for="u in overview.users" :key="u.id" class="hover:bg-[#FAFBFD] transition">
              <td class="py-2.5 px-5">
                <div class="flex items-center gap-2.5">
                  <img
                    :src="resolveAvatarUrl(u.avatar)"
                    :alt="u.name"
                    class="w-7 h-7 rounded-full object-cover border border-[#EAECEF] bg-white shrink-0"
                  />
                  <div class="min-w-0">
                    <p class="font-semibold text-[#18181B] truncate">{{ u.name }}</p>
                    <p class="text-[10px] text-[#94A3B8] truncate">{{ u.email || '未绑定邮箱' }}</p>
                  </div>
                </div>
              </td>
              <td class="py-2.5 px-3 text-right tabular-nums">{{ u.booksCount }} 本</td>
              <td class="py-2.5 px-3 text-right tabular-nums font-bold">
                {{ u.txCount.toLocaleString() }} 笔
              </td>
              <td class="py-2.5 px-3 text-right tabular-nums font-semibold text-[#DE8A1E]">
                {{ formatTokens(u.inputTokens + u.outputTokens) }}
              </td>
              <td class="py-2.5 px-5 text-right text-[#64748B]">{{ u.lastActive }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
