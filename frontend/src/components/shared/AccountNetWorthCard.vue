<script setup lang="ts">
import type { WalletSummary } from '@/types';
import { formatMoney } from '@/utils/format';

withDefaults(
  defineProps<{
    summary: WalletSummary;
    privacy?: boolean;
    currency?: string;
  }>(),
  {
    privacy: false,
    currency: 'CNY',
  }
);
</script>

<template>
  <div class="rounded-[22px] p-4 bg-[#18181B] text-white shadow-[0_6px_24px_-8px_rgba(15,23,42,0.35)]">
    <p class="text-[11px] text-white/65">净资产（{{ currency }}）</p>
    <p class="text-2xl font-bold tabular-nums mt-1">
      {{ formatMoney(summary?.net_assets ?? 0, { privacy }) }}
    </p>
    <div class="grid grid-cols-2 gap-3 mt-3 pt-3 border-t border-white/10 text-xs">
      <div>
        <p class="text-[10px] text-white/60">总资产</p>
        <p class="font-semibold tabular-nums mt-0.5 text-[#7AE0A8]">
          {{ formatMoney(summary?.total_assets ?? 0, { privacy }) }}
        </p>
      </div>
      <div>
        <p class="text-[10px] text-white/60">总负债</p>
        <p class="font-semibold tabular-nums mt-0.5 text-[#FCA5A5]">
          {{ formatMoney(summary?.total_liabilities ?? 0, { privacy }) }}
        </p>
      </div>
    </div>
  </div>
</template>
