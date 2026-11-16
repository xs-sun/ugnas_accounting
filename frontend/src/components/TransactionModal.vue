<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import type { Wallet, Category, Tag, TransactionType } from '../types';
import MorandiIcon from './MorandiIcon.vue';
import { api } from '../api/client';

const props = defineProps<{
  show: boolean;
  bookId: string;
  wallets: Wallet[];
  categories: Category[];
  tags: Tag[];
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'saved'): void;
}>();

const activeType = ref<TransactionType>('expense');
const amountInput = ref('');
const selectedWalletId = ref('');
const selectedToWalletId = ref('');
const selectedCategoryId = ref('');
const selectedDate = ref(new Date().toISOString().slice(0, 10));
const note = ref('');
const selectedTagIds = ref<string[]>([]);
const receiptFile = ref<File | null>(null);
const receiptPreview = ref<string | null>(null);
const submitting = ref(false);
const errorMessage = ref<string | null>(null);

// Reset form
const initForm = () => {
  activeType.value = 'expense';
  amountInput.value = '';
  selectedDate.value = new Date().toISOString().slice(0, 10);
  note.value = '';
  selectedTagIds.value = [];
  receiptFile.value = null;
  receiptPreview.value = null;
  errorMessage.value = null;

  if (props.wallets.length > 0) {
    selectedWalletId.value = props.wallets[0].id;
    if (props.wallets.length > 1) {
      selectedToWalletId.value = props.wallets[1].id;
    }
  }

  const defaultCat = filteredCategories.value[0];
  if (defaultCat) {
    selectedCategoryId.value = defaultCat.id;
  }
};

const filteredCategories = computed(() => {
  if (activeType.value === 'transfer') return [];
  return props.categories.filter((c) => c.type === activeType.value);
});

const switchType = (type: TransactionType) => {
  activeType.value = type;
  errorMessage.value = null;
  const cats = props.categories.filter((c) => c.type === (type === 'transfer' ? 'expense' : type));
  if (cats.length > 0) {
    selectedCategoryId.value = cats[0].id;
  } else {
    selectedCategoryId.value = '';
  }
};

const handleReceiptSelect = (e: Event) => {
  const input = e.target as HTMLInputElement;
  if (input.files && input.files[0]) {
    const f = input.files[0];
    receiptFile.value = f;
    receiptPreview.value = URL.createObjectURL(f);
  }
};

const removeReceipt = () => {
  receiptFile.value = null;
  receiptPreview.value = null;
};

const setQuickAmount = (val: number) => {
  amountInput.value = val.toString();
};

const toggleTag = (tagId: string) => {
  const idx = selectedTagIds.value.indexOf(tagId);
  if (idx >= 0) {
    selectedTagIds.value.splice(idx, 1);
  } else {
    selectedTagIds.value.push(tagId);
  }
};

const handleSave = async () => {
  errorMessage.value = null;
  const numVal = parseFloat(amountInput.value);
  if (isNaN(numVal) || numVal <= 0) {
    errorMessage.value = '请输入有效金额';
    return;
  }

  const cents = Math.round(numVal * 100);

  if (!selectedWalletId.value) {
    errorMessage.value = '请选择支付账户';
    return;
  }

  if (activeType.value === 'transfer') {
    if (!selectedToWalletId.value) {
      errorMessage.value = '请选择转入账户';
      return;
    }
    if (selectedWalletId.value === selectedToWalletId.value) {
      errorMessage.value = '转出账户与转入账户不能相同';
      return;
    }
  }

  submitting.value = true;
  try {
    const payload: any = {
      book_id: props.bookId,
      type: activeType.value,
      amount: cents,
      date: selectedDate.value,
      wallet_id: selectedWalletId.value,
      note: note.value,
      tags: selectedTagIds.value,
    };

    if (activeType.value === 'transfer') {
      payload.to_wallet_id = selectedToWalletId.value;
    } else if (selectedCategoryId.value) {
      payload.category_id = selectedCategoryId.value;
    }

    const created = await api.createTransaction(payload);

    // If receipt attached, upload it
    if (receiptFile.value && created.id) {
      await api.uploadAttachment(created.id, receiptFile.value);
    }

    emit('saved');
    emit('close');
  } catch (err: any) {
    errorMessage.value = err.message || '保存失败';
  } finally {
    submitting.value = false;
  }
};

onMounted(() => {
  initForm();
});
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/30 backdrop-blur-[2px] transition-opacity duration-200"
  >
    <!-- Modal Card (Squircle 20px / Mobile bottom drawer) -->
    <div
      class="w-full sm:max-w-lg bg-[#FFFFFF] rounded-t-[20px] sm:rounded-[20px] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] border border-[#EDEEF1] flex flex-col max-h-[92vh] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
    >
      <!-- Header -->
      <div class="px-6 pt-5 pb-3 flex items-center justify-between border-b border-[#EDEEF1]">
        <!-- Type Segmented Switcher -->
        <div class="flex items-center bg-[#F6F7F8] p-1 rounded-full border border-[#EDEEF1]">
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="
              activeType === 'expense'
                ? 'bg-[#E5654E] text-white shadow-xs'
                : 'text-[#5D6470] hover:text-[#171A20]'
            "
            @click="switchType('expense')"
          >
            支出
          </button>
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="
              activeType === 'income'
                ? 'bg-[#0E8C7F] text-white shadow-xs'
                : 'text-[#5D6470] hover:text-[#171A20]'
            "
            @click="switchType('income')"
          >
            收入
          </button>
          <button
            type="button"
            class="px-4 py-1.5 rounded-full text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="
              activeType === 'transfer'
                ? 'bg-[#171A20] text-white shadow-xs'
                : 'text-[#5D6470] hover:text-[#171A20]'
            "
            @click="switchType('transfer')"
          >
            转账
          </button>
        </div>

        <button
          type="button"
          class="w-8 h-8 rounded-full flex items-center justify-center text-[#9AA0AB] hover:text-[#171A20] hover:bg-[#F6F7F8] transition-colors cursor-pointer"
          @click="$emit('close')"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <!-- Scrollable Form Body -->
      <div class="p-6 overflow-y-auto space-y-5">
        <!-- Amount Input (Large Tabular Left-aligned) -->
        <div>
          <div class="flex items-baseline space-x-2 border-b-2 border-[#EDEEF1] pb-2 focus-within:border-[#0E8C7F] transition-colors">
            <span class="text-2xl font-bold text-[#171A20]">¥</span>
            <input
              v-model="amountInput"
              type="number"
              step="0.01"
              placeholder="0.00"
              autofocus
              class="w-full text-3xl font-bold text-[#171A20] tabular-nums tracking-tightest bg-transparent outline-hidden placeholder:text-[#9AA0AB]"
            />
          </div>

          <!-- Quick Amount Pills -->
          <div class="flex items-center space-x-2 mt-2.5">
            <button
              v-for="amt in [10, 20, 50, 100, 200, 500]"
              :key="amt"
              type="button"
              class="px-2.5 py-1 rounded-full bg-[#F6F7F8] hover:bg-[#EDEEF1] text-xs font-mono text-[#5D6470] transition-colors cursor-pointer"
              @click="setQuickAmount(amt)"
            >
              {{ amt }}
            </button>
          </div>
        </div>

        <!-- Wallets Selection -->
        <div v-if="activeType !== 'transfer'" class="space-y-1.5">
          <label class="text-xs font-medium text-[#5D6470]">支付账户</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="w in wallets"
              :key="w.id"
              type="button"
              class="flex items-center space-x-2 px-3 py-1.5 rounded-[11px] border text-xs font-medium transition-all duration-150 cursor-pointer"
              :class="
                selectedWalletId === w.id
                  ? 'border-[#0E8C7F] bg-[#E4F3F1] text-[#0E8C7F]'
                  : 'border-[#EDEEF1] bg-[#FFFFFF] text-[#171A20] hover:bg-[#F6F7F8]'
              "
              @click="selectedWalletId = w.id"
            >
              <MorandiIcon :icon="w.type" :size="20" />
              <span>{{ w.name }}</span>
            </button>
          </div>
        </div>

        <!-- Transfer Dual Wallet Selector -->
        <div v-else class="grid grid-cols-2 gap-3 bg-[#F6F7F8] p-3 rounded-[14px] border border-[#EDEEF1]">
          <div class="space-y-1">
            <label class="text-[11px] font-medium text-[#5D6470]">转出账户</label>
            <select
              v-model="selectedWalletId"
              class="w-full h-9 px-2 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            >
              <option v-for="w in wallets" :key="w.id" :value="w.id">{{ w.name }}</option>
            </select>
          </div>
          <div class="space-y-1">
            <label class="text-[11px] font-medium text-[#5D6470]">转入账户</label>
            <select
              v-model="selectedToWalletId"
              class="w-full h-9 px-2 text-xs bg-white rounded-[9px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            >
              <option v-for="w in wallets" :key="w.id" :value="w.id">{{ w.name }}</option>
            </select>
          </div>
        </div>

        <!-- Category Grid (For Expense / Income) -->
        <div v-if="activeType !== 'transfer'" class="space-y-2">
          <label class="text-xs font-medium text-[#5D6470]">选择分类</label>
          <div class="grid grid-cols-4 sm:grid-cols-5 gap-2 max-h-48 overflow-y-auto pr-1">
            <button
              v-for="cat in filteredCategories"
              :key="cat.id"
              type="button"
              class="flex flex-col items-center p-2 rounded-[14px] border transition-all duration-150 cursor-pointer"
              :class="
                selectedCategoryId === cat.id
                  ? 'border-[#0E8C7F] bg-[#E4F3F1]/50 shadow-xs'
                  : 'border-transparent hover:bg-[#F6F7F8]'
              "
              @click="selectedCategoryId = cat.id"
            >
              <MorandiIcon :icon="cat.icon || cat.name" :size="36" />
              <span class="text-xs text-[#171A20] font-medium mt-1 truncate w-full text-center">
                {{ cat.name }}
              </span>
            </button>
          </div>
        </div>

        <!-- Date & Note Inputs -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">记账日期</label>
            <input
              v-model="selectedDate"
              type="date"
              class="w-full h-10 px-3 text-xs bg-[#FFFFFF] rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            />
          </div>
          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">备注</label>
            <input
              v-model="note"
              type="text"
              placeholder="添加备注信息..."
              class="w-full h-10 px-3 text-xs bg-[#FFFFFF] rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            />
          </div>
        </div>

        <!-- Tags Selection -->
        <div v-if="tags.length > 0" class="space-y-1.5">
          <label class="text-xs font-medium text-[#5D6470]">标签</label>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="tg in tags"
              :key="tg.id"
              type="button"
              class="px-2.5 py-1 rounded-full text-xs font-medium border transition-colors cursor-pointer"
              :class="
                selectedTagIds.includes(tg.id)
                  ? 'bg-[#171A20] text-white border-[#171A20]'
                  : 'bg-[#F6F7F8] text-[#5D6470] border-[#EDEEF1] hover:text-[#171A20]'
              "
              @click="toggleTag(tg.id)"
            >
              #{{ tg.name }}
            </button>
          </div>
        </div>

        <!-- Receipt Attachment Upload -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium text-[#5D6470]">票据凭证</label>
          <div v-if="receiptPreview" class="relative inline-block">
            <img
              :src="receiptPreview"
              class="w-20 h-20 rounded-[11px] object-cover border border-[#EDEEF1]"
            />
            <button
              type="button"
              class="absolute -top-1.5 -right-1.5 w-5 h-5 rounded-full bg-[#E5654E] text-white text-xs flex items-center justify-center cursor-pointer shadow-xs"
              @click="removeReceipt"
            >
              ×
            </button>
          </div>
          <label
            v-else
            class="flex items-center justify-center space-x-2 w-full h-11 border border-dashed border-[#EDEEF1] rounded-[11px] hover:border-[#0E8C7F] hover:bg-[#E4F3F1]/20 text-xs text-[#5D6470] cursor-pointer transition-colors"
          >
            <svg class="w-4 h-4 text-[#9AA0AB]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <span>上传凭证票据图片 (.jpg, .png, .webp)</span>
            <input type="file" accept="image/jpeg,image/png,image/webp" class="hidden" @change="handleReceiptSelect" />
          </label>
        </div>

        <!-- Error Message Notice -->
        <div v-if="errorMessage" class="text-xs text-[#E5654E] bg-[#FDF1EE] p-2.5 rounded-[11px]">
          {{ errorMessage }}
        </div>
      </div>

      <!-- Footer Action -->
      <div class="p-6 pt-3 border-t border-[#EDEEF1] bg-[#FFFFFF]">
        <button
          type="button"
          class="w-full h-12 bg-[#0E8C7F] hover:bg-[#0B6E64] active:scale-[0.98] text-white font-medium text-sm rounded-full transition-all duration-150 flex items-center justify-center space-x-2 shadow-xs cursor-pointer disabled:opacity-50"
          :disabled="submitting"
          @click="handleSave"
        >
          <span v-if="submitting">保存中...</span>
          <span v-else>保存记账</span>
        </button>
      </div>
    </div>
  </div>
</template>
