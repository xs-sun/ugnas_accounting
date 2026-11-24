<script setup lang="ts">
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue';
import { api } from '@/api/client';
import { DEFAULT_MASCOT, MASCOT_FALLBACK } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import { centsToYuan, yuanToCents, todayIso, nowTimeHm } from '@/utils/format';
import type {
  AiChatMessage,
  AiDraft,
  Category,
  CategoryTreeNode,
  Transaction,
} from '@/types';
import CategoryBadge from './CategoryBadge.vue';

const props = withDefaults(
  defineProps<{
    mode?: 'mobile' | 'pc';
    bookId?: string;
    compact?: boolean;
  }>(),
  {
    mode: 'mobile',
    bookId: '',
    compact: false,
  }
);

const emit = defineEmits<{
  recorded: [tx: Transaction];
  'view-tx': [txId: string];
}>();

const store = useAppStore();

const targetBookId = computed(
  () => props.bookId || store.activeBookId || store.books[0]?.id || ''
);

const targetBookName = computed(() => {
  const found = store.books.find((b) => b.id === targetBookId.value);
  return found?.name || store.activeBook?.name || '当前账本';
});

const messagesRef = ref<HTMLElement | null>(null);
const inputText = ref('');
const sending = ref(false);
const savingMsgId = ref<string | null>(null);

// 语音与图片面板状态
const voiceOpen = ref(false);
const mediaTrayOpen = ref(false);
const voicePreviewText = ref('请说话，例如：“午饭吃拉面花了 32 元，微信支付”');
const voiceActiveText = ref('');
const isNativeSpeechActive = ref(false);

const VOICE_SAMPLES = [
  '午饭吃拉面花了 32 元，微信支付',
  '刚才在盒马买牛奶和面包花了 48.5 元，支付宝',
  '下班打车回家花了 26.8 元，微信支付',
];
let voiceSampleIdx = 0;
let voiceTypeTimer: ReturnType<typeof setInterval> | null = null;
let recognitionInstance: any = null;

const QUICK_PROMPTS = [
  { label: '☕ 瑞幸拿铁 18.5', prompt: '刚才在瑞幸买拿铁花了 18.5 元，微信支付' },
  { label: '🥬 买菜水果 64元', prompt: '晚饭买菜和水果一共 64 元，支付宝' },
  { label: '📊 本月餐饮花了多少？', prompt: '本月餐饮花了多少？' },
  { label: '🔍 支出最高的一笔？', prompt: '我本月支出最高的一笔是什么？' },
];

function scrollToBottom() {
  const el = messagesRef.value;
  if (el) {
    el.scrollTop = el.scrollHeight;
  }
}

watch(
  () => store.aiMessages.length,
  async () => {
    await nextTick();
    scrollToBottom();
  }
);

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

function getCategoryTree(type: 'expense' | 'income'): CategoryTreeNode[] {
  return type === 'income' ? store.incomeCategories : store.expenseCategories;
}

function getSubCategories(draft: AiDraft): Category[] {
  const tree = getCategoryTree(draft.type);
  const found = tree.find((c) => c.id === draft.category_id);
  return found?.children || [];
}

function formatDraftYuan(cents: number): string {
  return centsToYuan(cents).toFixed(2);
}

function onDraftAmountInput(draft: AiDraft, e: Event) {
  const val = (e.target as HTMLInputElement).value;
  const parsed = parseFloat(val);
  if (Number.isFinite(parsed) && parsed >= 0) {
    draft.amount = yuanToCents(parsed);
  } else if (val === '') {
    draft.amount = 0;
  }
}

function switchDraftType(draft: AiDraft, nextType: 'expense' | 'income') {
  if (draft.type === nextType) return;
  draft.type = nextType;
  const tree = getCategoryTree(nextType);
  if (tree.length > 0) {
    const first = tree[0];
    draft.category_id = first.id;
    draft.category_name = first.name;
    draft.category_icon = first.icon;
    draft.category_color = first.color;
    draft.bg_color = first.bg_color;
    draft.sub_category = first.children?.[0]?.name || '';
  }
}

function selectDraftCategory(draft: AiDraft, cat: CategoryTreeNode) {
  draft.category_id = cat.id;
  draft.category_name = cat.name;
  draft.category_icon = cat.icon;
  draft.category_color = cat.color;
  draft.bg_color = cat.bg_color;
  const subs = cat.children || [];
  if (!subs.some((s) => s.name === draft.sub_category)) {
    draft.sub_category = subs[0]?.name || '';
  }
}

function onDraftWalletChange(draft: AiDraft, e: Event) {
  const walletId = (e.target as HTMLSelectElement).value;
  const wallet = store.activeWallets.find((w) => w.id === walletId);
  if (wallet) {
    draft.wallet_id = wallet.id;
    draft.wallet_name = wallet.name;
  }
}

function toggleEditMessage(msg: AiChatMessage) {
  if (msg.saved) return;
  msg.editing = !msg.editing;
}

function updateUsageFromResponse(usage?: { today_count: number; daily_limit: number }) {
  if (usage && store.user) {
    store.user = {
      ...store.user,
      today_ai_count: usage.today_count,
      daily_ai_limit: usage.daily_limit,
    };
  }
}

async function sendPrompt(rawText: string, isVoice = false) {
  const text = rawText.trim();
  if (!text || sending.value) return;

  if (!targetBookId.value) {
    store.showToast('请先选择或创建账本', 'error');
    return;
  }

  store.appendAiMessage({
    id: `u_${Date.now()}`,
    role: 'user',
    text,
    isVoice,
  });

  sending.value = true;
  await nextTick();
  scrollToBottom();

  try {
    const res = await api.aiChat({
      book_id: targetBookId.value,
      message: text,
      is_voice: isVoice,
    });
    updateUsageFromResponse(res.usage);
    store.appendAiMessage({
      id: `a_${Date.now() + 1}`,
      role: 'assistant',
      text: res.reply,
      draft: res.draft,
      saved: false,
      editing: false,
    });
  } catch (err: any) {
    const errMsg = err?.message || 'AI 解析暂不可用，请稍后重试';
    store.appendAiMessage({
      id: `a_err_${Date.now() + 1}`,
      role: 'assistant',
      text: `抱歉，${errMsg}`,
      draft: null,
    });
    store.showToast(errMsg, 'error');
  } finally {
    sending.value = false;
    await nextTick();
    scrollToBottom();
  }
}

function handleFormSubmit() {
  if (!inputText.value.trim() || sending.value) return;
  const msg = inputText.value;
  inputText.value = '';
  sendPrompt(msg, false);
}

async function confirmRecordDraft(msg: AiChatMessage) {
  if (!msg.draft || msg.saved || savingMsgId.value) return;
  const d = msg.draft;
  if (!d.amount || d.amount <= 0) {
    store.showToast('请输入大于 0 的金额', 'error');
    return;
  }

  const walletId = d.wallet_id || store.activeWallets[0]?.id || '';
  if (!walletId) {
    store.showToast('当前账本暂无可用账户，请先添加账户', 'error');
    return;
  }

  savingMsgId.value = msg.id;
  try {
    const tx = await store.createTransaction({
      book_id: targetBookId.value,
      type: d.type,
      amount: Math.round(d.amount),
      date: d.date || todayIso(),
      tx_time: nowTimeHm(),
      wallet_id: walletId,
      category_id: d.category_id || undefined,
      title: d.title || d.category_name || 'AI 记账',
      sub_category: d.sub_category || '',
      note: d.note || '',
      include_budget: true,
    });

    msg.saved = true;
    msg.editing = false;
    msg.savedTxId = tx.id;

    store.showToast(
      `已记入「${targetBookName.value}」¥${centsToYuan(tx.amount).toFixed(2)}`,
      'success'
    );
    emit('recorded', tx);
  } catch (err: any) {
    store.showToast(err?.message || '入账失败，请检查数据', 'error');
  } finally {
    savingMsgId.value = null;
  }
}

// ==================== VOICE CAPTURE (Web Speech API + 演示句兜底) ====================

function stopVoiceEngine() {
  if (voiceTypeTimer) {
    clearInterval(voiceTypeTimer);
    voiceTypeTimer = null;
  }
  if (recognitionInstance) {
    try {
      recognitionInstance.stop();
    } catch {
      // ignore
    }
    recognitionInstance = null;
  }
  isNativeSpeechActive.value = false;
}

function startFallbackTypewriter() {
  const sample = VOICE_SAMPLES[voiceSampleIdx % VOICE_SAMPLES.length];
  voiceSampleIdx += 1;
  voiceActiveText.value = sample;
  voicePreviewText.value = '正在识别语音…';

  let idx = 0;
  if (voiceTypeTimer) clearInterval(voiceTypeTimer);
  voiceTypeTimer = setInterval(() => {
    idx += 2;
    voicePreviewText.value = sample.slice(0, idx);
    if (idx >= sample.length && voiceTypeTimer) {
      clearInterval(voiceTypeTimer);
      voiceTypeTimer = null;
    }
  }, 90);
}

function startVoiceCapture() {
  mediaTrayOpen.value = false;
  voiceOpen.value = true;
  stopVoiceEngine();

  const SpeechRec =
    (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;

  if (SpeechRec) {
    try {
      const rec = new SpeechRec();
      rec.lang = 'zh-CN';
      rec.interimResults = true;
      rec.continuous = true;
      recognitionInstance = rec;
      isNativeSpeechActive.value = true;
      voiceActiveText.value = '';
      voicePreviewText.value = '请开始说话，例如：“午饭吃拉面花了 32 元”…';

      rec.onresult = (event: any) => {
        let transcript = '';
        for (let i = 0; i < event.results.length; i++) {
          transcript += event.results[i][0]?.transcript || '';
        }
        if (transcript.trim()) {
          voiceActiveText.value = transcript.trim();
          voicePreviewText.value = transcript.trim();
        }
      };

      rec.onerror = () => {
        // 权限拒绝或离线无法连接语音服务时，优雅切换演示转写兜底
        if (!voiceActiveText.value) {
          stopVoiceEngine();
          startFallbackTypewriter();
        }
      };

      rec.start();
      return;
    } catch {
      // fallback below
    }
  }

  startFallbackTypewriter();
}

function toggleVoicePanel() {
  if (voiceOpen.value) {
    cancelVoice();
  } else {
    startVoiceCapture();
  }
}

function cancelVoice() {
  stopVoiceEngine();
  voiceOpen.value = false;
}

function finishVoice() {
  const spoken =
    voiceActiveText.value.trim() || VOICE_SAMPLES[0];
  stopVoiceEngine();
  voiceOpen.value = false;
  sendPrompt(spoken, true);
}

// ==================== OCR / IMAGE UPLOAD ====================

function toggleMediaTray() {
  if (voiceOpen.value) cancelVoice();
  mediaTrayOpen.value = !mediaTrayOpen.value;
}

function createReceiptSvgThumb(merchant: string, total: string, items: string[]): string {
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="280" height="190" viewBox="0 0 280 190">` +
    `<rect width="280" height="190" fill="#F8F9FB"/>` +
    `<rect x="24" y="14" width="232" height="162" rx="14" fill="#FFFFFF" stroke="#E2E8F0" stroke-width="2"/>` +
    `<text x="140" y="44" text-anchor="middle" font-size="14" font-weight="bold" fill="#18181B" font-family="sans-serif">${merchant}</text>` +
    `<line x1="42" y1="58" x2="238" y2="58" stroke="#E2E8F0" stroke-dasharray="4 4"/>` +
    `<text x="44" y="84" font-size="11" fill="#64748B" font-family="sans-serif">${items[0] || ''}</text>` +
    `<text x="44" y="106" font-size="11" fill="#64748B" font-family="sans-serif">${items[1] || ''}</text>` +
    `<line x1="42" y1="124" x2="238" y2="124" stroke="#E2E8F0" stroke-dasharray="4 4"/>` +
    `<text x="44" y="152" font-size="12" font-weight="bold" fill="#475569" font-family="sans-serif">实付合计</text>` +
    `<text x="236" y="152" text-anchor="end" font-size="16" font-weight="bold" fill="#D6553E" font-family="sans-serif">¥${total}</text>` +
    `</svg>`;
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

async function runOcrRequest(payload: {
  image: string;
  imageLabel: string;
  file?: File;
  demo?: string;
}) {
  if (sending.value) return;
  if (!targetBookId.value) {
    store.showToast('请先选择账本', 'error');
    return;
  }

  mediaTrayOpen.value = false;
  store.appendAiMessage({
    id: `u_img_${Date.now()}`,
    role: 'user',
    text: '',
    image: payload.image,
    imageLabel: payload.imageLabel,
  });

  sending.value = true;
  await nextTick();
  scrollToBottom();

  try {
    const res = await api.aiOcr({
      book_id: targetBookId.value,
      file: payload.file,
      demo: payload.demo,
    });
    updateUsageFromResponse(res.usage);
    store.appendAiMessage({
      id: `a_ocr_${Date.now() + 1}`,
      role: 'assistant',
      text: res.reply || '已识别图片中的小票与账单信息，点击下方预览卡可直接调整或确认入账：',
      draft: res.draft,
      saved: false,
      editing: false,
    });
  } catch (err: any) {
    const msg = err?.message || '小票识别失败，请稍后重试';
    store.appendAiMessage({
      id: `a_ocr_err_${Date.now() + 1}`,
      role: 'assistant',
      text: `抱歉，${msg}`,
      draft: null,
    });
    store.showToast(msg, 'error');
  } finally {
    sending.value = false;
    await nextTick();
    scrollToBottom();
  }
}

function handleOcrFileUpload(e: Event, sourceLabel: string) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = () => {
    runOcrRequest({
      image: String(reader.result || ''),
      imageLabel: sourceLabel,
      file,
    });
  };
  reader.readAsDataURL(file);
  input.value = '';
}

function runDemoOcr(kind: 'starbucks' | 'hema') {
  if (kind === 'starbucks') {
    const thumb = createReceiptSvgThumb('STARBUCKS 星巴克', '38.00', [
      '大杯焦糖玛奇朵 ×1       ¥38.00',
      '微信支付 · 电子小票',
    ]);
    runOcrRequest({
      image: thumb,
      imageLabel: '星巴克消费小票',
      demo: 'starbucks',
    });
  } else {
    const thumb = createReceiptSvgThumb('盒马鲜生 HEMA', '86.50', [
      '鲜牛奶 / 烘焙面包 / 蓝莓  ¥86.50',
      '支付宝免密支付',
    ]);
    runOcrRequest({
      image: thumb,
      imageLabel: '盒马鲜生购物清单',
      demo: 'hema',
    });
  }
}

onBeforeUnmount(() => {
  stopVoiceEngine();
});
</script>

<template>
  <div
    class="flex flex-col overflow-hidden"
    :class="
      compact
        ? 'h-[460px]'
        : mode === 'pc'
          ? 'flex-1 h-full min-h-[460px]'
          : 'flex-1 h-full min-h-[420px]'
    "
  >
    <!-- Message List -->
    <div
      ref="messagesRef"
      class="flex-1 overflow-y-auto px-4 py-3 space-y-3"
      :class="mode === 'pc' ? 'thin-scroll bg-[#F8F9FB]' : 'phone-scroll'"
    >
      <template v-for="m in store.aiMessages" :key="m.id">
        <!-- User Message -->
        <div v-if="m.role === 'user'" class="flex justify-end">
          <div class="max-w-[82%] space-y-1.5">
            <div
              v-if="m.image"
              class="rounded-2xl overflow-hidden border border-[#ECEEF2] bg-white shadow-xs"
            >
              <img
                :src="m.image"
                alt="识别图片"
                class="w-44 max-h-36 object-cover block"
              />
              <div
                class="px-2.5 py-1 bg-[#F8F9FB] text-[10px] text-[#64748B] flex items-center justify-between gap-2"
              >
                <span class="truncate">{{ m.imageLabel || '账单图片' }}</span>
                <span class="text-[#DE8A1E] font-semibold shrink-0">OCR 已识别</span>
              </div>
            </div>

            <div
              v-if="m.text"
              class="bg-[#18181B] text-white text-xs leading-relaxed px-3.5 py-2.5 rounded-2xl rounded-br-md shadow-xs"
            >
              <span
                v-if="m.isVoice"
                class="inline-flex items-center gap-1 text-[10px] text-[#F4B65B] font-semibold mr-1.5"
              >
                🎙 语音转文字
              </span>
              <span>{{ m.text }}</span>
            </div>
          </div>
        </div>

        <!-- Assistant Message + Interactive Draft Preview Card -->
        <div v-else class="flex items-start gap-2">
          <img
            :src="DEFAULT_MASCOT"
            alt="七猫助手"
            class="w-7 h-7 rounded-full object-contain bg-white p-0.5 shrink-0 mt-0.5 border border-[#ECEEF2]"
            @error="onMascotError"
          />

          <div class="flex-1 max-w-[86%] space-y-2 min-w-0">
            <div
              class="bg-white border border-[#ECEEF2] text-[#18181B] text-xs leading-relaxed px-3.5 py-2.5 rounded-2xl rounded-bl-md shadow-xs"
            >
              {{ m.text }}
            </div>

            <!-- Interactive Click-to-Adjust Preview Card -->
            <div
              v-if="m.draft"
              class="bg-white border rounded-2xl p-3 shadow-xs transition"
              :class="
                m.editing
                  ? 'border-[#DE8A1E] ring-2 ring-[#DE8A1E]/15'
                  : 'border-[#ECEEF2]'
              "
            >
              <!-- Clickable Summary Header -->
              <div
                class="select-none"
                :class="m.saved ? '' : 'cursor-pointer'"
                @click="toggleEditMessage(m)"
              >
                <div class="flex items-center justify-between mb-2">
                  <span
                    class="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                    :class="
                      m.saved
                        ? 'bg-[#ECF6F0] text-[#3E8E63]'
                        : 'bg-[#FFF6E8] text-[#DE8A1E]'
                    "
                  >
                    {{
                      m.saved
                        ? '✓ 已入账'
                        : m.editing
                          ? '正在调整预览'
                          : '点击卡片可调整'
                    }}
                  </span>
                  <span
                    class="text-sm font-bold tabular-nums"
                    :class="
                      m.draft.type === 'income'
                        ? 'text-[#3E8E63]'
                        : 'text-[#D6553E]'
                    "
                  >
                    {{ m.draft.type === 'income' ? '+' : '-' }}¥{{
                      formatDraftYuan(m.draft.amount)
                    }}
                  </span>
                </div>

                <div class="flex items-center justify-between gap-2.5">
                  <div class="flex items-center gap-2.5 min-w-0">
                    <CategoryBadge
                      :icon="m.draft.category_icon"
                      :color="m.draft.category_color"
                      :bg-color="m.draft.bg_color || '#FFF6E8'"
                      size="sm"
                    />
                    <div class="min-w-0">
                      <p class="text-xs font-bold text-[#18181B] truncate">
                        {{ m.draft.title }}
                      </p>
                      <p class="text-[11px] text-[#64748B] truncate mt-0.5">
                        {{ m.draft.category_name
                        }}{{
                          m.draft.sub_category ? ' · ' + m.draft.sub_category : ''
                        }}
                        · {{ m.draft.wallet_name || '默认账户' }} ·
                        {{ m.draft.date ? m.draft.date.slice(5) : '今天' }}
                      </p>
                    </div>
                  </div>

                  <span
                    v-if="!m.saved"
                    class="text-[11px] font-medium text-[#64748B] shrink-0 flex items-center gap-0.5"
                  >
                    <span>{{ m.editing ? '收起' : '调整' }}</span>
                    <svg
                      class="w-3.5 h-3.5 transition-transform"
                      :class="{ 'rotate-180': m.editing }"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path d="M6 9l6 6 6-6" />
                    </svg>
                  </span>
                </div>
              </div>

              <!-- Expandable Inline Adjustment Form -->
              <div
                v-if="!m.saved && m.editing"
                class="mt-3 pt-3 border-t border-[#F1F3F6] space-y-2.5 text-xs"
              >
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-[#94A3B8]">收支类型</span>
                  <div class="inline-flex p-0.5 rounded-full bg-[#F1F3F6] text-[11px] font-semibold">
                    <button
                      type="button"
                      class="px-3 py-0.5 rounded-full transition"
                      :class="
                        m.draft.type === 'expense'
                          ? 'bg-white text-[#D6553E] shadow-xs'
                          : 'text-[#64748B]'
                      "
                      @click="switchDraftType(m.draft, 'expense')"
                    >
                      支出
                    </button>
                    <button
                      type="button"
                      class="px-3 py-0.5 rounded-full transition"
                      :class="
                        m.draft.type === 'income'
                          ? 'bg-white text-[#3E8E63] shadow-xs'
                          : 'text-[#64748B]'
                      "
                      @click="switchDraftType(m.draft, 'income')"
                    >
                      收入
                    </button>
                  </div>
                </div>

                <div class="grid grid-cols-2 gap-2">
                  <div>
                    <label class="block text-[10px] text-[#94A3B8] mb-1">金额（元）</label>
                    <input
                      type="number"
                      step="0.01"
                      min="0.01"
                      :value="formatDraftYuan(m.draft.amount)"
                      class="w-full text-xs font-bold tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 outline-none focus:border-[#DE8A1E] focus:bg-white"
                      @input="onDraftAmountInput(m.draft, $event)"
                    />
                  </div>
                  <div>
                    <label class="block text-[10px] text-[#94A3B8] mb-1">名称 / 商户</label>
                    <input
                      v-model="m.draft.title"
                      type="text"
                      maxlength="24"
                      class="w-full text-xs font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 outline-none focus:border-[#DE8A1E] focus:bg-white"
                    />
                  </div>
                </div>

                <div v-if="getCategoryTree(m.draft.type).length > 0">
                  <label class="block text-[10px] text-[#94A3B8] mb-1">一级分类</label>
                  <div class="flex items-center gap-1 overflow-x-auto phone-scroll pb-0.5">
                    <button
                      v-for="cat in getCategoryTree(m.draft.type)"
                      :key="cat.id"
                      type="button"
                      class="shrink-0 text-[11px] px-2.5 py-1 rounded-full transition"
                      :class="
                        m.draft.category_id === cat.id
                          ? 'bg-[#18181B] text-white font-semibold'
                          : 'bg-[#F5F6F8] text-[#475569] hover:bg-[#ECEEF2]'
                      "
                      @click="selectDraftCategory(m.draft, cat)"
                    >
                      {{ cat.name }}
                    </button>
                  </div>
                </div>

                <div v-if="getSubCategories(m.draft).length > 0">
                  <label class="block text-[10px] text-[#94A3B8] mb-1">二级分类</label>
                  <div class="flex flex-wrap gap-1">
                    <button
                      v-for="sub in getSubCategories(m.draft)"
                      :key="sub.id"
                      type="button"
                      class="text-[11px] px-2 py-0.5 rounded-full transition"
                      :class="
                        m.draft.sub_category === sub.name
                          ? 'bg-[#FFF6E8] text-[#DE8A1E] font-semibold'
                          : 'bg-[#F5F6F8] text-[#64748B]'
                      "
                      @click="m.draft.sub_category = sub.name"
                    >
                      {{ sub.name }}
                    </button>
                  </div>
                </div>

                <div class="grid grid-cols-2 gap-2">
                  <div>
                    <label class="block text-[10px] text-[#94A3B8] mb-1">支付账户</label>
                    <select
                      :value="m.draft.wallet_id || store.activeWallets[0]?.id || ''"
                      class="w-full text-[11px] font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2 py-1.5 outline-none focus:border-[#DE8A1E]"
                      @change="onDraftWalletChange(m.draft, $event)"
                    >
                      <option
                        v-for="w in store.activeWallets"
                        :key="w.id"
                        :value="w.id"
                      >
                        {{ w.name }}
                      </option>
                    </select>
                  </div>
                  <div>
                    <label class="block text-[10px] text-[#94A3B8] mb-1">日期</label>
                    <input
                      v-model="m.draft.date"
                      type="date"
                      class="w-full text-[11px] font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2 py-1.5 outline-none focus:border-[#DE8A1E]"
                    />
                  </div>
                </div>
              </div>

              <!-- Footer Buttons -->
              <div v-if="!m.saved" class="mt-2.5 flex items-center gap-2">
                <button
                  type="button"
                  class="px-3 py-2 rounded-xl bg-[#F5F6F8] text-[#475569] text-xs font-semibold hover:bg-[#ECEEF2] transition shrink-0"
                  @click="toggleEditMessage(m)"
                >
                  {{ m.editing ? '完成调整' : '调整' }}
                </button>
                <button
                  type="button"
                  :disabled="savingMsgId === m.id"
                  class="flex-1 py-2 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition truncate px-2"
                  @click="confirmRecordDraft(m)"
                >
                  {{
                    savingMsgId === m.id
                      ? '入账中…'
                      : `确认记入「${targetBookName}」`
                  }}
                </button>
              </div>
              <button
                v-else-if="m.savedTxId"
                type="button"
                class="mt-2 w-full py-1.5 rounded-xl bg-[#F5F6F8] text-[#475569] text-[11px] font-semibold hover:bg-[#ECEEF2] transition"
                @click="emit('view-tx', m.savedTxId)"
              >
                查看 / 编辑已入账明细
              </button>
            </div>
          </div>
        </div>
      </template>

      <!-- Typing / Parsing Indicator -->
      <div v-if="sending" class="flex items-center gap-2">
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫助手"
          class="w-7 h-7 rounded-full object-contain bg-white p-0.5 shrink-0 border border-[#ECEEF2]"
          @error="onMascotError"
        />
        <div
          class="bg-white border border-[#ECEEF2] text-[#64748B] text-xs px-3.5 py-2 rounded-2xl rounded-bl-md shadow-xs"
        >
          小七正在思考解析中…
        </div>
      </div>
    </div>

    <!-- Bottom Input Dock -->
    <div
      class="shrink-0 space-y-2 border-t border-[#ECEEF2]/80"
      :class="mode === 'pc' ? 'p-4 bg-white' : 'px-3 pt-2 pb-4 bg-[#F8F9FB]'"
    >
      <!-- Quick Prompt Chips -->
      <div
        class="flex gap-1.5 overflow-x-auto pb-0.5"
        :class="mode === 'pc' ? 'thin-scroll' : 'phone-scroll'"
      >
        <button
          v-for="qp in QUICK_PROMPTS"
          :key="qp.label"
          type="button"
          :disabled="sending"
          class="shrink-0 text-[11px] px-3 py-1.5 rounded-full bg-white border border-[#E4E2D8] text-[#27272A] hover:border-[#DE8A1E] hover:text-[#DE8A1E] transition disabled:opacity-50"
          @click="sendPrompt(qp.prompt, false)"
        >
          {{ qp.label }}
        </button>
      </div>

      <!-- Photo / Camera OCR Tray -->
      <div
        v-if="mediaTrayOpen"
        class="bg-white border border-[#ECEEF2] rounded-2xl p-3 shadow-sm space-y-2.5"
      >
        <div class="flex items-center justify-between">
          <span class="text-[11px] font-bold text-[#18181B]">
            {{ mode === 'pc' ? '小票 / 截图 OCR 识别' : '图片与拍照识别记账' }}
          </span>
          <button
            type="button"
            class="text-[11px] text-[#94A3B8] hover:text-[#18181B]"
            @click="mediaTrayOpen = false"
          >
            收起
          </button>
        </div>

        <div
          class="grid gap-2"
          :class="mode === 'pc' ? 'grid-cols-1' : 'grid-cols-2'"
        >
          <label
            v-if="mode === 'mobile'"
            class="cursor-pointer flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-xl bg-[#F5F6F8] hover:bg-[#FFF6E8] text-xs font-semibold text-[#18181B] hover:text-[#DE8A1E] transition"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path
                d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"
              />
              <circle cx="12" cy="13" r="4" />
            </svg>
            <span>拍照识别小票</span>
            <input
              type="file"
              accept="image/*"
              capture="environment"
              class="hidden"
              @change="handleOcrFileUpload($event, '拍照识别小票')"
            />
          </label>

          <label
            class="cursor-pointer flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-xl bg-[#F5F6F8] hover:bg-[#FFF6E8] text-xs font-semibold text-[#18181B] hover:text-[#DE8A1E] transition"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="3" y="4" width="18" height="16" rx="2.5" />
              <circle cx="8.5" cy="9.5" r="1.5" />
              <path d="M21 15l-5-5L5 20" />
            </svg>
            <span>{{ mode === 'pc' ? '上传本地小票或账单截图' : '从相册选图' }}</span>
            <input
              type="file"
              accept="image/*"
              class="hidden"
              @change="handleOcrFileUpload($event, '相册账单图片')"
            />
          </label>
        </div>

        <div
          class="flex items-center gap-1.5 pt-1.5 border-t border-[#F1F3F6] overflow-x-auto phone-scroll"
        >
          <span class="text-[10px] text-[#94A3B8] shrink-0">模拟小票：</span>
          <button
            type="button"
            class="shrink-0 text-[11px] font-medium px-2.5 py-1 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition"
            @click="runDemoOcr('starbucks')"
          >
            星巴克小票 ¥38
          </button>
          <button
            type="button"
            class="shrink-0 text-[11px] font-medium px-2.5 py-1 rounded-full bg-[#F5F6F8] text-[#475569] hover:bg-[#ECEEF2] transition"
            @click="runDemoOcr('hema')"
          >
            盒马购物清单 ¥86.5
          </button>
        </div>
      </div>

      <!-- Voice Recording Waveform Panel -->
      <div
        v-if="voiceOpen"
        class="bg-white border border-[#DE8A1E]/40 rounded-2xl p-3 shadow-sm space-y-2.5"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full bg-[#D6553E] animate-pulse" />
            <span class="text-xs font-bold text-[#18181B]">
              {{ isNativeSpeechActive ? '正在聆听语音…' : '正在模拟语音识别…' }}
            </span>
          </div>
          <div class="flex items-center gap-1 h-4">
            <span
              class="voice-bar w-0.5 h-3.5 rounded-full bg-[#DE8A1E]"
              style="animation-delay: 0s"
            />
            <span
              class="voice-bar w-0.5 h-4 rounded-full bg-[#DE8A1E]"
              style="animation-delay: 0.15s"
            />
            <span
              class="voice-bar w-0.5 h-3 rounded-full bg-[#DE8A1E]"
              style="animation-delay: 0.3s"
            />
            <span
              class="voice-bar w-0.5 h-4 rounded-full bg-[#DE8A1E]"
              style="animation-delay: 0.45s"
            />
            <span
              class="voice-bar w-0.5 h-2.5 rounded-full bg-[#DE8A1E]"
              style="animation-delay: 0.6s"
            />
          </div>
        </div>
        <p
          class="text-xs text-[#475569] bg-[#F5F6F8] rounded-xl px-3 py-2 min-h-[34px] leading-relaxed"
        >
          {{ voicePreviewText }}
        </p>
        <div class="flex items-center justify-end gap-2">
          <button
            type="button"
            class="px-3 py-1.5 rounded-xl bg-[#F5F6F8] text-xs font-semibold text-[#64748B]"
            @click="cancelVoice"
          >
            取消
          </button>
          <button
            type="button"
            class="px-3.5 py-1.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] transition"
            @click="finishVoice"
          >
            完成并识别
          </button>
        </div>
      </div>

      <!-- Main Input Row -->
      <form
        class="flex items-center gap-1.5 bg-white border border-[#DFE3EA] rounded-full pl-3 pr-1.5 py-1.5 shadow-sm focus-within:border-[#DE8A1E]"
        @submit.prevent="handleFormSubmit"
      >
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫助手"
          class="w-6 h-6 rounded-full object-contain bg-white shrink-0"
          @error="onMascotError"
        />
        <input
          v-model="inputText"
          type="text"
          :disabled="sending"
          placeholder="说话、发图或打字记账/查账…"
          class="flex-1 min-w-0 text-xs bg-transparent outline-none text-[#18181B] placeholder-[#94A3B8]"
        />
        <button
          type="button"
          title="语音记账"
          class="w-8 h-8 rounded-full flex items-center justify-center transition shrink-0"
          :class="
            voiceOpen
              ? 'bg-[#FFF6E8] text-[#DE8A1E]'
              : 'bg-[#F5F6F8] text-[#475569] hover:bg-[#FFF6E8] hover:text-[#DE8A1E]'
          "
          @click="toggleVoicePanel"
        >
          <svg
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="9" y="3" width="6" height="11" rx="3" />
            <path d="M5 11a7 7 0 0 0 14 0" />
            <path d="M12 18v3M9 21h6" />
          </svg>
        </button>
        <button
          type="button"
          title="拍照或图片识别记账"
          class="w-8 h-8 rounded-full flex items-center justify-center transition shrink-0"
          :class="
            mediaTrayOpen
              ? 'bg-[#FFF6E8] text-[#DE8A1E]'
              : 'bg-[#F5F6F8] text-[#475569] hover:bg-[#FFF6E8] hover:text-[#DE8A1E]'
          "
          @click="toggleMediaTray"
        >
          <svg
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="4.5" width="18" height="15" rx="3" />
            <circle cx="8.5" cy="9.5" r="1.4" />
            <path d="M21 15l-4.5-4.5L6 19.5" />
          </svg>
        </button>
        <button
          type="submit"
          title="发送"
          :disabled="sending || !inputText.trim()"
          class="w-8 h-8 rounded-full bg-[#DE8A1E] text-white flex items-center justify-center hover:bg-[#C67612] disabled:opacity-50 transition shrink-0"
        >
          <svg
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
          >
            <path d="M12 19V5M5 12l7-7 7 7" />
          </svg>
        </button>
      </form>
    </div>
  </div>
</template>
