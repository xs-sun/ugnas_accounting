<script setup lang="ts">
import { ref, watch } from 'vue';
import type { Wallet, WalletType } from '../types';
import MorandiIcon from './MorandiIcon.vue';
import { api } from '../api/client';

const props = defineProps<{
  show: boolean;
  bookId: string;
  wallet?: Wallet | null;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'saved'): void;
}>();

const walletName = ref('');
const walletType = ref<WalletType>('debit');
const balanceInput = ref('0.00');
const creditLimitInput = ref('0.00');
const billingDay = ref(1);
const repaymentDay = ref(20);
const cardLastFour = ref('');
const excludeNetAsset = ref(false);
const submitting = ref(false);
const errorMessage = ref<string | null>(null);

const walletTypes: { type: WalletType; label: string }[] = [
  { type: 'wechat', label: '微信支付' },
  { type: 'alipay', label: '支付宝' },
  { type: 'debit', label: '储蓄卡' },
  { type: 'credit', label: '信用卡' },
  { type: 'cash', label: '现金' },
  { type: 'investment', label: '理财账户' },
  { type: 'custom', label: '其他/自定义' },
];

watch(
  () => props.show,
  (val) => {
    if (val) {
      errorMessage.value = null;
      if (props.wallet) {
        walletName.value = props.wallet.name;
        walletType.value = props.wallet.type;
        balanceInput.value = (props.wallet.balance / 100).toFixed(2);
        creditLimitInput.value = (props.wallet.credit_limit / 100).toFixed(2);
        billingDay.value = props.wallet.billing_day || 1;
        repaymentDay.value = props.wallet.repayment_day || 20;
        cardLastFour.value = props.wallet.card_last_four || '';
        excludeNetAsset.value = props.wallet.exclude_net_asset;
      } else {
        walletName.value = '';
        walletType.value = 'debit';
        balanceInput.value = '0.00';
        creditLimitInput.value = '0.00';
        billingDay.value = 1;
        repaymentDay.value = 20;
        cardLastFour.value = '';
        excludeNetAsset.value = false;
      }
    }
  },
  { immediate: true }
);

const handleSave = async () => {
  errorMessage.value = null;
  const nameTrim = walletName.value.trim();
  if (!nameTrim) {
    errorMessage.value = '账户名称不能为空';
    return;
  }

  const balFloat = parseFloat(balanceInput.value);
  if (isNaN(balFloat)) {
    errorMessage.value = '请输入有效余额';
    return;
  }
  const balCents = Math.round(balFloat * 100);

  const limitFloat = parseFloat(creditLimitInput.value);
  const limitCents = isNaN(limitFloat) ? 0 : Math.round(limitFloat * 100);

  if (walletType.value === 'credit') {
    if (billingDay.value < 1 || billingDay.value > 31) {
      errorMessage.value = '账单日需在 1 到 31 之间';
      return;
    }
    if (repaymentDay.value < 1 || repaymentDay.value > 31) {
      errorMessage.value = '还款日需在 1 到 31 之间';
      return;
    }
    if (cardLastFour.value && !/^\d{4}$/.test(cardLastFour.value)) {
      errorMessage.value = '卡号后四位需为 4 位数字';
      return;
    }
  }

  submitting.value = true;
  try {
    const payload: any = {
      book_id: props.bookId,
      name: nameTrim,
      type: walletType.value,
      balance: balCents,
      credit_limit: limitCents,
      billing_day: billingDay.value,
      repayment_day: repaymentDay.value,
      card_last_four: cardLastFour.value,
      exclude_net_asset: excludeNetAsset.value,
    };

    if (props.wallet?.id) {
      await api.updateWallet(props.wallet.id, payload);
    } else {
      await api.createWallet(payload);
    }

    emit('saved');
    emit('close');
  } catch (err: any) {
    errorMessage.value = err.message || '保存失败';
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/30 backdrop-blur-[2px] transition-opacity"
  >
    <div
      class="w-full sm:max-w-md bg-[#FFFFFF] rounded-t-[20px] sm:rounded-[20px] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] border border-[#EDEEF1] flex flex-col overflow-hidden"
    >
      <!-- Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-[#EDEEF1]">
        <h3 class="text-base font-semibold text-[#171A20]">
          {{ wallet ? '编辑钱包账户' : '添加钱包账户' }}
        </h3>
        <button
          type="button"
          class="w-8 h-8 rounded-full flex items-center justify-center text-[#9AA0AB] hover:text-[#171A20] hover:bg-[#F6F7F8] cursor-pointer"
          @click="$emit('close')"
        >
          ✕
        </button>
      </div>

      <!-- Form Body -->
      <div class="p-6 space-y-4 max-h-[80vh] overflow-y-auto">
        <!-- Account Type Selection -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium text-[#5D6470]">账户类型</label>
          <div class="grid grid-cols-3 sm:grid-cols-4 gap-2">
            <button
              v-for="item in walletTypes"
              :key="item.type"
              type="button"
              class="flex flex-col items-center p-2 rounded-[11px] border text-center transition-all cursor-pointer"
              :class="
                walletType === item.type
                  ? 'border-[#0E8C7F] bg-[#E4F3F1]/40 text-[#0E8C7F]'
                  : 'border-[#EDEEF1] text-[#5D6470] hover:bg-[#F6F7F8]'
              "
              @click="walletType = item.type"
            >
              <MorandiIcon :icon="item.type" :size="28" />
              <span class="text-xs font-medium mt-1 truncate w-full">{{ item.label }}</span>
            </button>
          </div>
        </div>

        <!-- Name & Balance -->
        <div class="space-y-1">
          <label class="text-xs font-medium text-[#5D6470]">账户名称</label>
          <input
            v-model="walletName"
            type="text"
            placeholder="例如: 工商银行储蓄卡、微信零钱..."
            class="w-full h-10 px-3 text-xs bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs font-medium text-[#5D6470]">
            {{ walletType === 'credit' ? '当前已用额度 / 欠款 (元)' : '当前余额 (元)' }}
          </label>
          <input
            v-model="balanceInput"
            type="number"
            step="0.01"
            class="w-full h-10 px-3 text-sm font-semibold tabular-nums text-[#171A20] bg-white rounded-[11px] border border-[#EDEEF1] outline-hidden focus:border-[#0E8C7F]"
          />
        </div>

        <!-- Credit Card Specific Settings -->
        <div v-if="walletType === 'credit'" class="space-y-3 p-3.5 bg-[#F6F7F8] rounded-[14px] border border-[#EDEEF1]">
          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">信用总额度 (元)</label>
            <input
              v-model="creditLimitInput"
              type="number"
              step="0.01"
              class="w-full h-9 px-3 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1">
              <label class="text-xs font-medium text-[#5D6470]">账单日 (1-31)</label>
              <input
                v-model="billingDay"
                type="number"
                min="1"
                max="31"
                class="w-full h-9 px-3 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs font-medium text-[#5D6470]">还款日 (1-31)</label>
              <input
                v-model="repaymentDay"
                type="number"
                min="1"
                max="31"
                class="w-full h-9 px-3 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
              />
            </div>
          </div>

          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">卡号后四位</label>
            <input
              v-model="cardLastFour"
              type="text"
              maxlength="4"
              placeholder="例如: 8820"
              class="w-full h-9 px-3 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] font-mono outline-hidden focus:border-[#0E8C7F]"
            />
          </div>
        </div>

        <!-- Exclude from Net Asset Toggle -->
        <label class="flex items-center justify-between p-3 bg-[#F6F7F8] rounded-[11px] cursor-pointer">
          <div class="text-xs">
            <div class="font-medium text-[#171A20]">不计入净资产</div>
            <div class="text-[#9AA0AB]">该账户余额将不影响总资产与净资产统计</div>
          </div>
          <input
            v-model="excludeNetAsset"
            type="checkbox"
            class="w-4 h-4 text-[#0E8C7F] rounded-[4px] border-[#EDEEF1] focus:ring-[#0E8C7F]"
          />
        </label>

        <!-- Error Alert -->
        <div v-if="errorMessage" class="text-xs text-[#E5654E] bg-[#FDF1EE] p-2.5 rounded-[11px]">
          {{ errorMessage }}
        </div>
      </div>

      <!-- Footer Action -->
      <div class="p-6 pt-3 border-t border-[#EDEEF1]">
        <button
          type="button"
          class="w-full h-11 bg-[#0E8C7F] hover:bg-[#0B6E64] active:scale-[0.98] text-white font-medium text-xs rounded-full transition-all duration-150 flex items-center justify-center space-x-2 cursor-pointer disabled:opacity-50"
          :disabled="submitting"
          @click="handleSave"
        >
          <span v-if="submitting">保存中...</span>
          <span v-else>保存账户</span>
        </button>
      </div>
    </div>
  </div>
</template>
