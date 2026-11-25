<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { api } from '@/api/client';
import { DEFAULT_MASCOT, MASCOT_FALLBACK, resolveAvatarUrl } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import { centsToYuan, yuanToCents, todayIso, nowTimeHm } from '@/utils/format';
import type {
  AiChatMessage,
  AiDraft,
  BookMember,
  Category,
  CategoryTreeNode,
  Transaction,
} from '@/types';
import CategoryBadge from './CategoryBadge.vue';
import MemberPickerSheet from './MemberPickerSheet.vue';

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

// 微信同款语音与文本输入状态
type VoiceCaptureMode = 'recorder' | 'speech' | 'unavailable';
type HoldActionZone = 'send' | 'cancel' | 'transcribe';

const voiceInputMode = ref(false); // false=文本输入栏, true=微信「按住 说话」按钮
const voiceOverlayOpen = ref(false);
const mediaTrayOpen = ref(false);

// 草稿调整：账户 / 日期用自定义底部弹出层，不用原生 select / date 输入
// （原生控件样式和记账页设计对不上）。
type PickerKind = 'wallet' | 'date';
const pickerOpen = ref<PickerKind | null>(null);
const pickerDraft = ref<AiDraft | null>(null);
const calYear = ref(0);
const calMonth = ref(0); // 0-11
// 草稿归属成员：默认是我，可点开换成其他真实/虚拟成员；入账时随草稿一起提交。
const memberPickerDraft = ref<AiDraft | null>(null);
const myMember = computed<BookMember | undefined>(() => {
  const members = store.activeBook?.members || [];
  return members.find((m) => m.is_me) || members[0];
});

function draftMember(draft: AiDraft): BookMember | undefined {
  const members = store.activeBook?.members || [];
  if (draft.member_id) return members.find((m) => m.user_id === draft.member_id);
  return myMember.value;
}

function openMemberPicker(draft: AiDraft) {
  memberPickerDraft.value = draft;
}

function onSelectDraftMember(userId: string) {
  const d = memberPickerDraft.value;
  if (d) {
    d.member_id = userId;
    const m = store.activeBook?.members?.find((x) => x.user_id === userId);
    d.member_name = m?.user_name || '';
  }
  memberPickerDraft.value = null;
}

// OCR 查看器：点上传的图片，看原图 + 本地 OCR 认出的文字。
const ocrViewer = ref<{ image: string; text: string; label: string } | null>(null);
const voiceMode = ref<VoiceCaptureMode>('speech');
const holdTarget = ref<HoldActionZone>('send');
const voiceStatusText = ref('松手 发语音');const voiceErrorText = ref('');
const voiceActiveText = ref('');
const voiceTranscribing = ref(false);
const polishingText = ref(false);
const polishedNotice = ref('');
const isRecordingAudio = ref(false);
const isNativeSpeechActive = ref(false);
const recordingSeconds = ref(0);
const waveLevels = ref<number[]>([24, 38, 52, 70, 44, 84, 60, 92, 56, 78, 42, 64, 34]);
const textInputRef = ref<HTMLTextAreaElement | null>(null);
const holdBtnRef = ref<HTMLElement | null>(null);
const cancelZoneRef = ref<HTMLElement | null>(null);
const transcribeZoneRef = ref<HTMLElement | null>(null);
const sttConfigured = ref<boolean | null>(null);
const sttModelName = ref('');

let recognitionInstance: any = null;
let mediaRecorder: MediaRecorder | null = null;
let mediaStream: MediaStream | null = null;
let recordedChunks: Blob[] = [];
let recordedMimeType = '';
let audioCtx: AudioContext | null = null;
let meterTimer: ReturnType<typeof setInterval> | null = null;
let durationTimer: ReturnType<typeof setInterval> | null = null;
let shouldStopWithoutSend = false;
let activePointerId: number | null = null;
let pressStartTime = 0;
let pressStartX = 0;
let pressStartY = 0;
let isHoldPressed = false;

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

function formatDraftDate(dateStr?: string): string {
  if (!dateStr) return '选择日期';
  const m = dateStr.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return dateStr;
  return `${m[1]}年${Number(m[2])}月${Number(m[3])}日`;
}

function openWalletPicker(draft: AiDraft) {
  pickerDraft.value = draft;
  pickerOpen.value = 'wallet';
}

function openDatePicker(draft: AiDraft) {
  pickerDraft.value = draft;
  const d = draft.date ? new Date(draft.date + 'T00:00:00') : new Date();
  calYear.value = d.getFullYear();
  calMonth.value = d.getMonth();
  pickerOpen.value = 'date';
}

function closePicker() {
  pickerOpen.value = null;
  pickerDraft.value = null;
}

function selectPickerWallet(wallet: { id: string; name: string }) {
  const d = pickerDraft.value;
  if (d) {
    d.wallet_id = wallet.id;
    d.wallet_name = wallet.name;
  }
  closePicker();
}

function selectPickerDate(day: number) {
  const d = pickerDraft.value;
  if (d) {
    d.date = `${calYear.value}-${String(calMonth.value + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
  }
  closePicker();
}

function selectTodayDate() {
  const d = pickerDraft.value;
  if (d) d.date = todayIso();
  closePicker();
}

function shiftCalMonth(delta: number) {
  const m = calMonth.value + delta;
  calYear.value = calYear.value + Math.floor(m / 12);
  calMonth.value = ((m % 12) + 12) % 12;
}

// 月历格子：周一起始，前面补空白格。
const calCells = computed(() => {
  const first = new Date(calYear.value, calMonth.value, 1);
  const startWeekday = (first.getDay() + 6) % 7;
  const daysInMonth = new Date(calYear.value, calMonth.value + 1, 0).getDate();
  const cells: { day: number; inMonth: boolean; dateStr: string }[] = [];
  for (let i = 0; i < startWeekday; i++) cells.push({ day: 0, inMonth: false, dateStr: '' });
  for (let day = 1; day <= daysInMonth; day++) {
    cells.push({
      day,
      inMonth: true,
      dateStr: `${calYear.value}-${String(calMonth.value + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`,
    });
  }
  return cells;
});

function openOcrViewer(msg: AiChatMessage) {
  if (!msg.image) return;
  ocrViewer.value = { image: msg.image, text: msg.ocrText || '', label: msg.imageLabel || '账单图片' };
}

function closeOcrViewer() {
  ocrViewer.value = null;
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
      member_id: d.member_id || undefined,
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

// ==================== 微信同款按住说话 & 转文字 AI 润色 ====================

function getSpeechRecConstructor(): any {
  if (typeof window === 'undefined') return null;
  return (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition || null;
}

function canUseMediaRecorder(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof MediaRecorder !== 'undefined' &&
    Boolean(navigator.mediaDevices?.getUserMedia)
  );
}

const formattedRecordingTime = computed(() => {
  const sec = recordingSeconds.value;
  const mm = String(Math.floor(sec / 60)).padStart(2, '0');
  const ss = String(sec % 60).padStart(2, '0');
  return `${mm}:${ss}`;
});

async function refreshAiCapabilities(): Promise<boolean> {
  try {
    const caps = await api.getAiCapabilities();
    sttConfigured.value = Boolean(caps.stt_configured);
    sttModelName.value = caps.stt_model || '';
    return Boolean(caps.stt_configured);
  } catch {
    if (sttConfigured.value === null) {
      sttConfigured.value = false;
    }
    return Boolean(sttConfigured.value);
  }
}

onMounted(() => {
  void refreshAiCapabilities();
});

function autoResizeTextInput() {
  const el = textInputRef.value;
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = `${Math.min(84, Math.max(24, el.scrollHeight))}px`;
}

function handleTextKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    handleFormSubmit();
  }
}

function toggleVoiceInputMode() {
  if (sending.value || voiceTranscribing.value) return;
  mediaTrayOpen.value = false;
  voiceInputMode.value = !voiceInputMode.value;
  if (!voiceInputMode.value) {
    void nextTick(() => {
      autoResizeTextInput();
      textInputRef.value?.focus();
    });
  } else {
    void refreshAiCapabilities();
  }
}

function cleanupAudioMeters() {
  if (meterTimer) {
    clearInterval(meterTimer);
    meterTimer = null;
  }
  if (durationTimer) {
    clearInterval(durationTimer);
    durationTimer = null;
  }
  if (audioCtx) {
    try {
      void audioCtx.close();
    } catch {
      // ignore
    }
    audioCtx = null;
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((t) => {
      try {
        t.stop();
      } catch {
        // ignore
      }
    });
    mediaStream = null;
  }
}

function stopVoiceEngine() {
  shouldStopWithoutSend = true;
  if (recognitionInstance) {
    try {
      recognitionInstance.onresult = null;
      recognitionInstance.onerror = null;
      recognitionInstance.onend = null;
      recognitionInstance.stop();
    } catch {
      // ignore
    }
    recognitionInstance = null;
  }
  if (mediaRecorder) {
    if (mediaRecorder.state !== 'inactive') {
      try {
        mediaRecorder.stop();
      } catch {
        // ignore
      }
    }
    mediaRecorder = null;
  }
  cleanupAudioMeters();
  recordedChunks = [];
  isNativeSpeechActive.value = false;
  isRecordingAudio.value = false;
  recordingSeconds.value = 0;
  waveLevels.value = [24, 38, 52, 70, 44, 84, 60, 92, 56, 78, 42, 64, 34];
}

function pickSupportedAudioMimeType(): string {
  if (typeof MediaRecorder === 'undefined' || typeof MediaRecorder.isTypeSupported !== 'function') {
    return '';
  }
  const candidates = [
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/mp4;codecs=mp4a.40.2',
    'audio/mp4',
    'audio/ogg;codecs=opus',
  ];
  return candidates.find((t) => MediaRecorder.isTypeSupported(t)) || '';
}

function extensionForMime(mime: string): string {
  const lower = (mime || '').toLowerCase();
  if (lower.includes('mp4') || lower.includes('aac') || lower.includes('m4a')) return 'm4a';
  if (lower.includes('ogg')) return 'ogg';
  if (lower.includes('wav')) return 'wav';
  return 'webm';
}

function fallbackToKeyboardDictation(notice: string) {
  stopVoiceEngine();
  voiceOverlayOpen.value = false;
  voiceInputMode.value = false;
  polishedNotice.value = notice;
  void nextTick(() => {
    autoResizeTextInput();
    textInputRef.value?.focus();
  });
}

async function startAudioRecorderCapture(): Promise<boolean> {
  if (!canUseMediaRecorder()) return false;
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    if (!isHoldPressed) {
      stream.getTracks().forEach((t) => t.stop());
      return false;
    }
    mediaStream = stream;
    recordedChunks = [];
    recordedMimeType = pickSupportedAudioMimeType();
    const rec = recordedMimeType
      ? new MediaRecorder(stream, { mimeType: recordedMimeType })
      : new MediaRecorder(stream);
    if (!recordedMimeType && rec.mimeType) {
      recordedMimeType = rec.mimeType;
    }
    rec.ondataavailable = (ev: BlobEvent) => {
      if (ev.data && ev.data.size > 0) {
        recordedChunks.push(ev.data);
      }
    };

    shouldStopWithoutSend = false;
    mediaRecorder = rec;
    voiceMode.value = 'recorder';
    isRecordingAudio.value = true;
    recordingSeconds.value = 0;
    voiceErrorText.value = '';

    durationTimer = setInterval(() => {
      recordingSeconds.value += 1;
    }, 1000);

    try {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
      if (AudioCtx) {
        audioCtx = new AudioCtx();
        const source = audioCtx.createMediaStreamSource(stream);
        const analyser = audioCtx.createAnalyser();
        analyser.fftSize = 32;
        source.connect(analyser);
        const dataArr = new Uint8Array(analyser.frequencyBinCount);
        meterTimer = setInterval(() => {
          analyser.getByteFrequencyData(dataArr);
          const next = Array.from({ length: 13 }, (_, idx) => {
            const sym = idx <= 6 ? idx : 12 - idx;
            const raw = dataArr[sym % dataArr.length] || 0;
            const boost = 20 + Math.round((raw / 255) * 78);
            return Math.min(100, Math.max(18, boost));
          });
          waveLevels.value = next;
        }, 85);
      }
    } catch {
      // 无法读取频率数据时保留静态波动
    }

    rec.start(200);
    return true;
  } catch (err: any) {
    const name = err?.name || '';
    if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
      voiceErrorText.value = '未授予麦克风权限，可点击输入框按键盘「🎙 语音输入」说话';
    } else if (!window.isSecureContext) {
      voiceErrorText.value = '当前非 HTTPS 访问限制网页麦克风，可点输入框用键盘「🎙 语音输入」';
    } else {
      voiceErrorText.value = `无法开启录音（${err?.message || '设备不可用'}），可用键盘「🎙 语音输入」`;
    }
    voiceMode.value = 'unavailable';
    return false;
  }
}

function startBrowserSpeechCapture(): boolean {
  const SpeechRec = getSpeechRecConstructor();
  if (!SpeechRec) return false;

  try {
    const rec = new SpeechRec();
    const isIOS =
      typeof navigator !== 'undefined' &&
      /iP(hone|ad|od)|Macintosh.*Mobile/i.test(navigator.userAgent);
    rec.lang = 'zh-CN';
    rec.interimResults = true;
    rec.continuous = !isIOS;

    let committedText = '';
    recognitionInstance = rec;
    voiceMode.value = 'speech';
    isNativeSpeechActive.value = true;
    voiceErrorText.value = '';

    rec.onresult = (event: any) => {
      let interim = '';
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const chunk = event.results[i][0]?.transcript || '';
        if (event.results[i].isFinal) {
          committedText = `${committedText} ${chunk}`.trim();
        } else {
          interim += chunk;
        }
      }
      const full = `${committedText}${interim ? (committedText ? ' ' : '') + interim : ''}`.trim();
      if (full) {
        voiceActiveText.value = full;
      }
    };

    rec.onerror = (ev: any) => {
      const code = ev?.error || '';
      isNativeSpeechActive.value = false;
      recognitionInstance = null;
      if (code === 'aborted') return;
      if (code === 'not-allowed' || code === 'service-not-allowed') {
        voiceErrorText.value = '语音识别权限未开启，松手后可用键盘「🎙 语音输入」说话';
      } else if (code === 'network') {
        voiceErrorText.value = '内置语音服务连网失败；可在后台配置 Whisper/SenseVoice 或用键盘语音';
      } else if (code === 'no-speech') {
        voiceErrorText.value = '未检测到人声';
      } else {
        voiceErrorText.value = `当前环境不支持网页直接识音（${code || '受限'}），请用键盘「🎙 语音输入」`;
      }
      if (code !== 'no-speech') {
        voiceMode.value = 'unavailable';
      }
    };

    rec.onend = () => {
      isNativeSpeechActive.value = false;
      recognitionInstance = null;
    };

    rec.start();
    return true;
  } catch {
    return false;
  }
}

function pointInEl(el: HTMLElement | null, x: number, y: number, pad = 18): boolean {
  if (!el) return false;
  const r = el.getBoundingClientRect();
  return x >= r.left - pad && x <= r.right + pad && y >= r.top - pad && y <= r.bottom + pad;
}

function detectHoldZone(x: number, y: number): HoldActionZone {
  if (pointInEl(cancelZoneRef.value, x, y, 20)) {
    return 'cancel';
  }
  if (pointInEl(transcribeZoneRef.value, x, y, 20)) {
    return 'transcribe';
  }
  const dy = y - pressStartY;
  const dx = x - pressStartX;
  if (dy < -42) {
    if (dx < -45) return 'cancel';
    if (dx > 45) return 'transcribe';
  }
  return 'send';
}

function detachGlobalHoldListeners() {
  if (typeof window === 'undefined') return;
  window.removeEventListener('pointermove', onGlobalHoldMove);
  window.removeEventListener('pointerup', onGlobalHoldUp);
  window.removeEventListener('pointercancel', onGlobalHoldCancel);
}

function onGlobalHoldMove(e: PointerEvent) {
  if (!isHoldPressed) return;
  if (activePointerId !== null && e.pointerId !== activePointerId) return;
  holdTarget.value = detectHoldZone(e.clientX, e.clientY);
}

function onGlobalHoldCancel(e: PointerEvent) {
  if (!isHoldPressed) return;
  if (activePointerId !== null && e.pointerId !== activePointerId) return;
  isHoldPressed = false;
  activePointerId = null;
  detachGlobalHoldListeners();
  stopVoiceEngine();
  voiceOverlayOpen.value = false;
}

async function onHoldVoicePointerDown(e: PointerEvent) {
  if (sending.value || voiceTranscribing.value || isHoldPressed) return;
  if (e.button !== undefined && e.button !== 0) return;

  e.preventDefault();
  mediaTrayOpen.value = false;
  isHoldPressed = true;
  activePointerId = e.pointerId;
  pressStartTime = Date.now();
  pressStartX = e.clientX;
  pressStartY = e.clientY;
  holdTarget.value = 'send';
  voiceActiveText.value = '';
  voiceErrorText.value = '';
  voiceOverlayOpen.value = true;

  if (typeof window !== 'undefined') {
    window.addEventListener('pointermove', onGlobalHoldMove, { passive: true });
    window.addEventListener('pointerup', onGlobalHoldUp);
    window.addEventListener('pointercancel', onGlobalHoldCancel);
  }
  try {
    (e.currentTarget as HTMLElement | null)?.setPointerCapture?.(e.pointerId);
  } catch {
    // ignore
  }

  const hasCloudStt = sttConfigured.value ?? (await refreshAiCapabilities());
  if (!isHoldPressed) return;

  if (hasCloudStt && canUseMediaRecorder()) {
    await startAudioRecorderCapture();
    return;
  }

  if (startBrowserSpeechCapture()) {
    return;
  }

  voiceMode.value = 'unavailable';
  voiceErrorText.value =
    canUseMediaRecorder() && !hasCloudStt
      ? '未配置云端语音转写模型且当前浏览器无内置识音；滑向右侧「转文字」可切回输入框直接用键盘「🎙 语音输入」说话并让 AI 优化！'
      : '当前环境限制网页调用麦克风；滑向右侧「转文字」切回输入框按键盘「🎙 语音输入」即可。';
}

function stopRecorderAndBuildBlob(): Promise<{ blob: Blob; filename: string } | null> {
  return new Promise((resolve) => {
    const rec = mediaRecorder;
    if (!rec || rec.state === 'inactive') {
      const mime = recordedMimeType || 'audio/webm';
      const blob = recordedChunks.length ? new Blob(recordedChunks, { type: mime }) : null;
      cleanupAudioMeters();
      mediaRecorder = null;
      isRecordingAudio.value = false;
      resolve(blob ? { blob, filename: `voice.${extensionForMime(mime)}` } : null);
      return;
    }

    rec.onstop = () => {
      const mime = recordedMimeType || rec.mimeType || 'audio/webm';
      const chunks = [...recordedChunks];
      cleanupAudioMeters();
      mediaRecorder = null;
      isRecordingAudio.value = false;
      if (shouldStopWithoutSend || chunks.length === 0) {
        resolve(null);
        return;
      }
      const blob = new Blob(chunks, { type: mime });
      resolve({ blob, filename: `voice.${extensionForMime(mime)}` });
    };

    try {
      rec.stop();
    } catch {
      cleanupAudioMeters();
      mediaRecorder = null;
      isRecordingAudio.value = false;
      resolve(null);
    }
  });
}

async function polishIntoTextInput(rawText: string, noticeLabel: string) {
  let finalText = rawText.trim();
  if (!finalText) return;
  try {
    voiceStatusText.value = 'AI 正在润色优化语音文本…';
    const res = await api.aiPolish(finalText);
    updateUsageFromResponse(res.usage);
    if (res.text?.trim()) {
      finalText = res.text.trim();
    }
  } catch {
    // 若润色请求失败，保留原始识别文字供用户继续编辑
  }
  inputText.value = finalText;
  voiceInputMode.value = false;
  polishedNotice.value = noticeLabel;
  void nextTick(() => {
    autoResizeTextInput();
    textInputRef.value?.focus();
  });
}

async function onGlobalHoldUp(e: PointerEvent) {
  if (!isHoldPressed) return;
  if (activePointerId !== null && e.pointerId !== activePointerId) return;

  const zone = detectHoldZone(e.clientX, e.clientY);
  const elapsedMs = Date.now() - pressStartTime;
  const currentMode = voiceMode.value;

  isHoldPressed = false;
  activePointerId = null;
  detachGlobalHoldListeners();

  // 1. 左滑取消
  if (zone === 'cancel') {
    stopVoiceEngine();
    voiceOverlayOpen.value = false;
    store.showToast('已取消语音输入', 'info');
    return;
  }

  // 2. 当前网页环境不支持直接识音：转入键盘语音输入 + AI 润色引导（不编造假句子）
  if (currentMode === 'unavailable') {
    if (zone === 'transcribe') {
      fallbackToKeyboardDictation('已切回输入模式：点击输入框后按键盘「🎙 语音输入」转成文字，再点「✨ AI 优化」即可继续编辑或发送');
    } else {
      fallbackToKeyboardDictation('当前环境未开放网页录音识音，请按键盘「🎙 语音输入」说话（说完可点 ✨ AI 优化）');
      store.showToast(voiceErrorText.value || '请使用键盘语音输入', 'info');
    }
    return;
  }

  // 3. 说话时间过短保护
  if (elapsedMs < 350 && !voiceActiveText.value.trim()) {
    stopVoiceEngine();
    voiceOverlayOpen.value = false;
    store.showToast('说话时间太短，请按住说话', 'info');
    return;
  }

  // 4. 真实录音模式 -> 上传后端 Whisper / SenseVoice 转写
  if (currentMode === 'recorder') {
    shouldStopWithoutSend = false;
    voiceTranscribing.value = true;
    voiceStatusText.value =
      zone === 'transcribe' ? '正在转文字并由 AI 优化…' : '正在识别语音内容…';
    const captured = await stopRecorderAndBuildBlob();
    if (!captured || captured.blob.size < 256) {
      voiceTranscribing.value = false;
      voiceOverlayOpen.value = false;
      store.showToast('未采集到清晰声音，请重试', 'info');
      return;
    }

    try {
      const res = await api.aiTranscribe(captured.blob, captured.filename, {
        polish: zone === 'transcribe',
      });
      updateUsageFromResponse(res.usage);
      const text = (res.text || '').trim();
      voiceTranscribing.value = false;
      voiceOverlayOpen.value = false;
      stopVoiceEngine();

      if (!text) {
        store.showToast('未听清语音内容，请重试', 'info');
        return;
      }

      if (zone === 'transcribe') {
        inputText.value = text;
        voiceInputMode.value = false;
        polishedNotice.value = '✨ 已转文字并由 AI 优化，可继续编辑或直接发送';
        void nextTick(() => {
          autoResizeTextInput();
          textInputRef.value?.focus();
        });
      } else {
        void sendPrompt(text, true);
      }
    } catch (err: any) {
      voiceTranscribing.value = false;
      stopVoiceEngine();
      fallbackToKeyboardDictation(
        `${err?.message || '语音转写失败'}，可改用键盘「🎙 语音输入」说话`
      );
      store.showToast(err?.message || '语音转写失败', 'error');
    }
    return;
  }

  // 5. 浏览器原生实时语音识别模式
  const capturedSpeech = voiceActiveText.value.trim();
  stopVoiceEngine();
  if (!capturedSpeech) {
    voiceOverlayOpen.value = false;
    if (zone === 'transcribe') {
      fallbackToKeyboardDictation('未识别到语音，可点击输入框用键盘「🎙 语音输入」并点 ✨ AI 优化');
    } else {
      store.showToast('没有听清您说的话，请按住重试', 'info');
    }
    return;
  }

  if (zone === 'transcribe') {
    voiceTranscribing.value = true;
    try {
      await polishIntoTextInput(capturedSpeech, '✨ 已转文字并由 AI 优化，可继续编辑或直接发送');
    } finally {
      voiceTranscribing.value = false;
      voiceOverlayOpen.value = false;
    }
  } else {
    voiceOverlayOpen.value = false;
    void sendPrompt(capturedSpeech, true);
  }
}

async function handlePolishInputText() {
  const raw = inputText.value.trim();
  if (!raw || polishingText.value || sending.value) return;
  polishingText.value = true;
  try {
    const res = await api.aiPolish(raw);
    updateUsageFromResponse(res.usage);
    if (res.text?.trim()) {
      inputText.value = res.text.trim();
      polishedNotice.value = '✨ AI 已优化文本表述，可继续修改或直接发送';
      void nextTick(() => {
        autoResizeTextInput();
        textInputRef.value?.focus();
      });
    }
  } catch (err: any) {
    store.showToast(err?.message || 'AI 优化文本失败', 'error');
  } finally {
    polishingText.value = false;
  }
}

function handleFormSubmit() {
  if (!inputText.value.trim() || sending.value) return;
  const msg = inputText.value;
  inputText.value = '';
  polishedNotice.value = '';
  void nextTick(() => autoResizeTextInput());
  void sendPrompt(msg, false);
}

// ==================== OCR / IMAGE UPLOAD ====================

function toggleMediaTray() {
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
  const userMsg: AiChatMessage = {
    id: `u_img_${Date.now()}`,
    role: 'user',
    text: '',
    image: payload.image,
    imageLabel: payload.imageLabel,
  };
  store.appendAiMessage(userMsg);

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
    // 识别文字留给图片查看器：点图片能看到 OCR 原文。
    if (res.raw_text) userMsg.ocrText = res.raw_text;
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

// 上传前先压缩图片：手机原图动辄几 MB，base64 后更大，视觉模型读得慢、
// 容易超时失败。压到最长边 1568px、JPEG 0.85，小票文字足够清晰。
// 解码失败（如 HEIC）时退回原文件，不挡路。
async function compressImageFile(file: File): Promise<File> {
  try {
    const bitmap = await createImageBitmap(file);
    const maxEdge = 1568;
    const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height));
    if (scale >= 1 && file.type === 'image/jpeg' && file.size <= 1024 * 1024) {
      bitmap.close();
      return file;
    }
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(bitmap.width * scale));
    canvas.height = Math.max(1, Math.round(bitmap.height * scale));
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      bitmap.close();
      return file;
    }
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    bitmap.close();
    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob((b) => resolve(b), 'image/jpeg', 0.85);
    });
    if (!blob) return file;
    return new File([blob], file.name.replace(/\.[^.]+$/, '') + '.jpg', { type: 'image/jpeg' });
  } catch {
    return file;
  }
}

async function handleOcrFileUpload(e: Event, sourceLabel: string) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  input.value = '';

  const uploadFile = await compressImageFile(file);
  const reader = new FileReader();
  reader.onload = () => {
    runOcrRequest({
      image: String(reader.result || ''),
      imageLabel: sourceLabel,
      file: uploadFile,
    });
  };
  reader.readAsDataURL(uploadFile);
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
  detachGlobalHoldListeners();
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
              class="rounded-2xl overflow-hidden border border-[#ECEEF2] bg-white shadow-xs cursor-pointer active:scale-[0.98] transition"
              @click="openOcrViewer(m)"
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
                <span class="text-[#DE8A1E] font-semibold shrink-0">{{
                  m.ocrText ? '查看识别结果 ›' : 'OCR 已识别'
                }}</span>
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
                    <button
                      type="button"
                      class="w-full text-[11px] font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 flex items-center justify-between gap-1 active:bg-[#FFF6E8]"
                      @click="openWalletPicker(m.draft)"
                    >
                      <span class="truncate">{{
                        m.draft.wallet_name || store.activeWallets[0]?.name || '选择账户'
                      }}</span>
                      <svg
                        class="w-3.5 h-3.5 shrink-0 text-[#94A3B8]"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path d="M6 9l6 6 6-6" />
                      </svg>
                    </button>
                  </div>
                  <div>
                    <label class="block text-[10px] text-[#94A3B8] mb-1">日期</label>
                    <button
                      type="button"
                      class="w-full text-[11px] font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 flex items-center justify-between gap-1 active:bg-[#FFF6E8]"
                      @click="openDatePicker(m.draft)"
                    >
                      <span class="truncate">{{ formatDraftDate(m.draft.date) }}</span>
                      <svg
                        class="w-3.5 h-3.5 shrink-0 text-[#94A3B8]"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <rect x="3.5" y="5" width="17" height="15.5" rx="3.5" />
                        <path d="M16 3v3.5M8 3v3.5M3.5 10h17" />
                      </svg>
                    </button>
                  </div>
                </div>

                <div class="mt-2">
                  <label class="block text-[10px] text-[#94A3B8] mb-1">归属成员</label>
                  <button
                    type="button"
                    class="w-full text-[11px] font-semibold bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 flex items-center justify-between gap-1 active:bg-[#FFF6E8]"
                    @click="openMemberPicker(m.draft)"
                  >
                    <span class="flex items-center gap-1.5 min-w-0">
                      <img
                        v-if="draftMember(m.draft)?.avatar"
                        :src="resolveAvatarUrl(draftMember(m.draft)?.avatar || '')"
                        :alt="draftMember(m.draft)?.user_name"
                        class="w-4 h-4 rounded-full object-cover shrink-0"
                      />
                      <span
                        v-else
                        class="w-4 h-4 rounded-full text-[8px] font-bold text-white flex items-center justify-center shrink-0"
                        :style="{ backgroundColor: draftMember(m.draft)?.color || '#DE8A1E' }"
                      >
                        {{ (draftMember(m.draft)?.user_name || '?').slice(0, 1) }}
                      </span>
                      <span class="truncate">{{
                        draftMember(m.draft)?.user_name || '选择成员'
                      }}</span>
                      <span
                        v-if="draftMember(m.draft)?.is_virtual"
                        class="text-[9px] font-medium px-1 py-0.5 rounded-full bg-[#EEF2FF] text-[#4F46E5] shrink-0"
                      >
                        虚拟成员
                      </span>
                    </span>
                    <svg
                      class="w-3.5 h-3.5 shrink-0 text-[#94A3B8]"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path d="M6 9l6 6 6-6" />
                    </svg>
                  </button>
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

      <!-- 语音转文字 / AI 润色提示条 -->
      <div
        v-if="polishedNotice || (!voiceInputMode && inputText.trim().length >= 3)"
        class="flex items-center justify-between gap-2 px-3 py-1.5 rounded-xl bg-[#FFFDF8] border border-[#EFE6D2] text-[11px]"
      >
        <span class="text-[#78716C] truncate">
          {{ polishedNotice || '可通过键盘语音或打字录入，再点击右侧按钮让 AI 润色表述' }}
        </span>
        <div class="flex items-center gap-1.5 shrink-0">
          <button
            v-if="inputText.trim().length >= 2"
            type="button"
            :disabled="polishingText || sending"
            class="px-2 py-0.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] font-semibold hover:bg-[#FBEBD1] disabled:opacity-50 transition"
            @click="handlePolishInputText"
          >
            {{ polishingText ? '优化中…' : '✨ AI 优化' }}
          </button>
          <button
            v-if="polishedNotice"
            type="button"
            class="text-[#94A3B8] hover:text-[#475569] px-1"
            @click="polishedNotice = ''"
          >
            ×
          </button>
        </div>
      </div>

      <!-- 微信同款主输入栏（键盘输入 <-> 按住 说话 一键切换） -->
      <form
        class="flex items-end gap-1.5"
        @submit.prevent="handleFormSubmit"
      >
        <!-- 左侧：语音 / 键盘 切换按钮 -->
        <button
          type="button"
          :title="voiceInputMode ? '切换为打字输入' : '切换为按住说话'"
          class="w-9 h-9 rounded-full bg-white border border-[#DFE3EA] flex items-center justify-center text-[#18181B] hover:border-[#DE8A1E] hover:text-[#DE8A1E] shadow-xs transition shrink-0"
          @click="toggleVoiceInputMode"
        >
          <!-- 键盘图标（处于按住说话模式时展示） -->
          <svg
            v-if="voiceInputMode"
            class="w-[18px] h-[18px]"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="2.5" y="5" width="19" height="14" rx="3" />
            <path d="M6.5 9.5h.01M10 9.5h.01M13.5 9.5h.01M17 9.5h.01M6.5 13h.01M10 13h.01M13.5 13h.01M17 13h.01M8 16h8" />
          </svg>
          <!-- 语音波形/麦克风图标（处于文字输入模式时展示） -->
          <svg
            v-else
            class="w-[18px] h-[18px]"
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

        <!-- 中间：按住 说话 按钮 或 自适应多行文本框 -->
        <div
          v-if="voiceInputMode"
          ref="holdBtnRef"
          role="button"
          tabindex="0"
          class="flex-1 min-w-0 h-9 rounded-full border shadow-xs flex items-center justify-center text-xs font-bold tracking-wide select-none touch-none transition [-webkit-touch-callout:none] [-webkit-user-select:none]"
          :class="
            voiceOverlayOpen
              ? 'bg-[#ECEEF2] border-[#CBD5E1] text-[#18181B] scale-[0.99]'
              : 'bg-white border-[#DFE3EA] text-[#18181B] active:bg-[#ECEEF2]'
          "
          @pointerdown="onHoldVoicePointerDown"
          @contextmenu.prevent
        >
          {{ voiceOverlayOpen ? '松开 结束' : '按住 说话' }}
        </div>

        <div
          v-else
          class="flex-1 min-w-0 flex items-end gap-1.5 bg-white border border-[#DFE3EA] rounded-[20px] pl-3 pr-2 py-1.5 shadow-xs focus-within:border-[#DE8A1E]"
        >
          <img
            :src="DEFAULT_MASCOT"
            alt="七猫助手"
            class="w-6 h-6 rounded-full object-contain bg-white shrink-0 mb-0.5"
            @error="onMascotError"
          />
          <textarea
            ref="textInputRef"
            v-model="inputText"
            rows="1"
            :disabled="sending"
            placeholder="打字、点🎙说话或发小票图记账…"
            class="flex-1 min-w-0 text-xs bg-transparent outline-none text-[#18181B] placeholder-[#94A3B8] resize-none leading-6 max-h-[84px] py-0"
            @input="autoResizeTextInput"
            @keydown="handleTextKeydown"
          />
        </div>

        <!-- 右侧：拍照 / 小票识别按钮 -->
        <button
          type="button"
          title="拍照或图片识别记账"
          class="w-9 h-9 rounded-full bg-white border flex items-center justify-center shadow-xs transition shrink-0"
          :class="
            mediaTrayOpen
              ? 'border-[#DE8A1E] bg-[#FFF6E8] text-[#DE8A1E]'
              : 'border-[#DFE3EA] text-[#475569] hover:border-[#DE8A1E] hover:text-[#DE8A1E]'
          "
          @click="toggleMediaTray"
        >
          <svg
            class="w-[18px] h-[18px]"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="12" cy="12" r="9" />
            <path d="M12 8.5v7M8.5 12h7" />
          </svg>
        </button>

        <!-- 发送按钮（仅文字模式显示） -->
        <button
          v-if="!voiceInputMode"
          type="submit"
          title="发送"
          :disabled="sending || !inputText.trim()"
          class="w-9 h-9 rounded-full bg-[#DE8A1E] text-white flex items-center justify-center hover:bg-[#C67612] disabled:opacity-45 shadow-xs transition shrink-0"
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

    <!-- 微信同款长按录音全屏 HUD 浮层（左滑取消 · 松手发送 · 右滑转文字并 AI 优化） -->
    <Teleport to="body">
      <div
        v-if="voiceOverlayOpen"
        class="fixed inset-0 z-[95] bg-[#14171A]/75 backdrop-blur-[3px] flex flex-col justify-between select-none touch-none [-webkit-user-select:none]"
        @contextmenu.prevent
      >
        <!-- 上半部分：留白 + 居中悬浮声波气泡 -->
        <div class="flex-1 flex flex-col items-center justify-center px-6 pt-12">
          <!-- 浏览器原生识音实时字幕预览（有内容时展示） -->
          <div
            v-if="voiceActiveText.trim()"
            class="mb-4 max-w-[300px] px-3.5 py-2 rounded-2xl bg-white/95 text-[#18181B] text-xs leading-relaxed shadow-lg text-center"
          >
            {{ voiceActiveText }}
          </div>

          <!-- 环境提示（仅无语音权限/接口时提示） -->
          <div
            v-else-if="voiceErrorText"
            class="mb-4 max-w-[300px] px-3.5 py-2 rounded-2xl bg-[#FFF8EB] border border-[#F5DFB5] text-[#92400E] text-[11px] leading-relaxed shadow-md text-center"
          >
            {{ voiceErrorText }}
          </div>

          <!-- 居中浮层主气泡（带向下小三角，高度还原微信） -->
          <div class="relative flex flex-col items-center">
            <div
              class="min-w-[172px] max-w-[270px] min-h-[76px] px-5 py-4 rounded-[22px] shadow-2xl flex items-center justify-center transition-all duration-150"
              :class="
                holdTarget === 'cancel'
                  ? 'bg-[#FA5151] text-white scale-95'
                  : holdTarget === 'transcribe'
                    ? 'bg-white text-[#18181B] scale-105'
                    : 'bg-[#95EC69] text-[#1A2E05]'
              "
            >
              <template v-if="voiceTranscribing">
                <div class="flex items-center gap-2 text-xs font-bold text-[#18181B]">
                  <span class="w-4 h-4 rounded-full border-2 border-[#DE8A1E] border-t-transparent animate-spin" />
                  <span>{{ voiceStatusText }}</span>
                </div>
              </template>
              <template v-else-if="holdTarget === 'cancel'">
                <div class="flex items-center gap-1.5 text-sm font-bold">
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4">
                    <path d="M18 6L6 18M6 6l12 12" />
                  </svg>
                  <span>取消发送</span>
                </div>
              </template>
              <template v-else-if="holdTarget === 'transcribe'">
                <div class="text-center space-y-1">
                  <p class="text-xs font-bold text-[#18181B]">转文字 · AI 润色优化</p>
                  <p class="text-[10px] text-[#64748B]">松手后可继续编辑或发送</p>
                </div>
              </template>
              <template v-else>
                <!-- 居中对称动态声波竖条 -->
                <div class="flex items-center justify-center gap-[3.5px] h-9">
                  <span
                    v-for="(lv, idx) in waveLevels"
                    :key="idx"
                    class="w-[2.5px] rounded-full bg-current/80 transition-all duration-100"
                    :class="{ 'voice-bar': isNativeSpeechActive && !isRecordingAudio }"
                    :style="
                      isRecordingAudio
                        ? { height: `${Math.max(22, lv)}%` }
                        : { height: `${30 + ((idx * 17) % 55)}%`, animationDelay: `${(idx % 5) * 0.12}s` }
                    "
                  />
                </div>
              </template>
            </div>
            <!-- 气泡底部小三角 -->
            <div
              class="w-3.5 h-3.5 -mt-2 rotate-45 rounded-[2px] transition-colors duration-150"
              :class="
                holdTarget === 'cancel'
                  ? 'bg-[#FA5151]'
                  : holdTarget === 'transcribe'
                    ? 'bg-white'
                    : 'bg-[#95EC69]'
              "
            />
          </div>

          <!-- 气泡下方状态提示文字 -->
          <p class="mt-5 text-xs font-medium text-white/85 tracking-wide tabular-nums">
            <template v-if="voiceTranscribing">
              {{ voiceStatusText }}
            </template>
            <template v-else-if="holdTarget === 'cancel'">
              松手 取消发送
            </template>
            <template v-else-if="holdTarget === 'transcribe'">
              松手 转文字并 AI 优化
            </template>
            <template v-else-if="voiceMode === 'unavailable'">
              左滑取消 · 右滑切回输入并用键盘🎙转文字
            </template>
            <template v-else>
              松手 直接识别发送{{ isRecordingAudio ? ` · ${formattedRecordingTime}` : '' }}
            </template>
          </p>
        </div>

        <!-- 下半部分：左右扇形操作热区（取消 / 转文字） + 底部收音弧形底座 -->
        <div class="relative w-full overflow-hidden pt-4">
          <div class="flex items-end justify-between px-5 pb-7 gap-4">
            <!-- 左侧热区：取消 -->
            <div
              ref="cancelZoneRef"
              class="w-[42%] max-w-[168px] h-[68px] rounded-[34px] flex flex-col items-center justify-center transition-all duration-150 -rotate-6 border"
              :class="
                holdTarget === 'cancel'
                  ? 'bg-[#FA5151] border-[#FF7878] text-white scale-105 shadow-xl'
                  : 'bg-white/15 border-white/10 text-white/85'
              "
            >
              <span class="text-xs font-bold">取消</span>
            </div>

            <!-- 右侧热区：转文字并优化 -->
            <div
              ref="transcribeZoneRef"
              class="w-[42%] max-w-[168px] h-[68px] rounded-[34px] flex flex-col items-center justify-center transition-all duration-150 rotate-6 border"
              :class="
                holdTarget === 'transcribe'
                  ? 'bg-white border-white text-[#18181B] scale-105 shadow-xl'
                  : 'bg-white/15 border-white/10 text-white/85'
              "
            >
              <span class="text-xs font-bold">
                {{ holdTarget === 'transcribe' ? '转文字 (AI优化)' : '滑到这里 转文字' }}
              </span>
            </div>
          </div>

          <!-- 最底部浅灰拱形收音底座 -->
          <div class="relative h-[86px] w-full flex items-center justify-center">
            <div
              class="w-[130%] -mx-[15%] h-[170px] rounded-t-[100%] border-t transition-colors duration-150 flex flex-col items-center pt-5"
              :class="
                holdTarget === 'send'
                  ? 'bg-gradient-to-b from-[#ECEEF2] to-[#CBD5E1] border-white/60 text-[#334155]'
                  : 'bg-gradient-to-b from-[#3A3F45] to-[#23272B] border-white/15 text-white/50'
              "
            >
              <svg
                class="w-5 h-5"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M2 10v3M6 6v11M10 3v18M14 8v7M18 5v13M22 10v3" />
              </svg>
              <span class="text-[10px] font-semibold mt-1">
                {{ holdTarget === 'send' ? '正在收音区' : '已移出发送区' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 账户选择：底部弹出层（不用原生 select） -->
    <div
      v-if="pickerOpen === 'wallet'"
      class="fixed inset-0 z-[100] flex items-end justify-center"
    >
      <div class="absolute inset-0 bg-[#14171A]/50" @click="closePicker" />
      <div
        class="relative w-full max-w-md bg-white rounded-t-3xl px-4 pt-2 pb-6 max-h-[70vh] overflow-y-auto phone-scroll"
      >
        <div class="w-10 h-1 rounded-full bg-[#E6E9EF] mx-auto my-2" />
        <p class="text-sm font-bold text-[#18181B] text-center mb-3">选择支付账户</p>
        <button
          v-for="w in store.activeWallets"
          :key="w.id"
          type="button"
          class="w-full flex items-center justify-between px-3 py-3 rounded-xl text-sm transition"
          :class="
            pickerDraft?.wallet_id === w.id || (!pickerDraft?.wallet_id && store.activeWallets[0]?.id === w.id)
              ? 'bg-[#FFF6E8] text-[#DE8A1E] font-semibold'
              : 'text-[#18181B] hover:bg-[#F5F6F8]'
          "
          @click="selectPickerWallet(w)"
        >
          <span>{{ w.name }}</span>
          <span
            v-if="pickerDraft?.wallet_id === w.id || (!pickerDraft?.wallet_id && store.activeWallets[0]?.id === w.id)"
            >✓</span
          >
        </button>
      </div>
    </div>

    <!-- 日期选择：底部弹出层·月历（不用原生 date 输入） -->
    <div
      v-if="pickerOpen === 'date'"
      class="fixed inset-0 z-[100] flex items-end justify-center"
    >
      <div class="absolute inset-0 bg-[#14171A]/50" @click="closePicker" />
      <div class="relative w-full max-w-md bg-white rounded-t-3xl px-4 pt-2 pb-6">
        <div class="w-10 h-1 rounded-full bg-[#E6E9EF] mx-auto my-2" />
        <div class="flex items-center justify-between mb-2">
          <button
            type="button"
            class="w-8 h-8 rounded-full bg-[#F5F6F8] text-[#475569] text-base leading-none"
            @click="shiftCalMonth(-1)"
          >
            ‹
          </button>
          <p class="text-sm font-bold text-[#18181B]">{{ calYear }}年{{ calMonth + 1 }}月</p>
          <button
            type="button"
            class="w-8 h-8 rounded-full bg-[#F5F6F8] text-[#475569] text-base leading-none"
            @click="shiftCalMonth(1)"
          >
            ›
          </button>
        </div>
        <div class="grid grid-cols-7 text-center text-[11px] text-[#94A3B8] mb-1">
          <span v-for="wd in ['一', '二', '三', '四', '五', '六', '日']" :key="wd">{{ wd }}</span>
        </div>
        <div class="grid grid-cols-7 gap-0.5">
          <button
            v-for="(cell, i) in calCells"
            :key="i"
            type="button"
            :disabled="!cell.inMonth"
            class="h-9 rounded-full text-xs transition"
            :class="
              cell.inMonth
                ? pickerDraft?.date === cell.dateStr
                  ? 'bg-[#DE8A1E] text-white font-bold'
                  : 'text-[#18181B] hover:bg-[#F5F6F8]'
                : 'text-transparent'
            "
            @click="cell.inMonth && selectPickerDate(cell.day)"
          >
            {{ cell.day || '' }}
          </button>
        </div>
        <button
          type="button"
          class="w-full mt-3 py-2 rounded-xl bg-[#F5F6F8] text-xs font-semibold text-[#475569]"
          @click="selectTodayDate"
        >
          回到今天
        </button>
      </div>
    </div>

    <!-- OCR 查看器：原图 + 识别文字 -->
    <div
      v-if="ocrViewer"
      class="fixed inset-0 z-[100] bg-[#14171A]/85 flex flex-col"
      @click="closeOcrViewer"
    >
      <div class="flex items-center justify-between px-4 pt-12 pb-2 text-white shrink-0">
        <span class="text-sm font-semibold truncate">{{ ocrViewer.label }}</span>
        <button
          type="button"
          class="w-8 h-8 shrink-0 rounded-full bg-white/15 text-lg leading-none"
          @click="closeOcrViewer"
        >
          ×
        </button>
      </div>
      <div class="flex-1 overflow-y-auto px-4 pb-8 space-y-3 phone-scroll" @click.stop>
        <img :src="ocrViewer.image" alt="账单图片" class="w-full rounded-2xl" />
        <div class="bg-white rounded-2xl p-3">
          <p class="text-[11px] font-semibold text-[#94A3B8] mb-1.5">OCR 识别文字</p>
          <p
            v-if="ocrViewer.text"
            class="text-xs leading-relaxed text-[#18181B] whitespace-pre-wrap break-words"
          >
            {{ ocrViewer.text }}
          </p>
          <p v-else class="text-xs text-[#94A3B8]">这次没有返回识别文字。</p>
        </div>
      </div>
    </div>
  </div>

    <!-- 草稿归属成员：底部弹出层（与手动记账共用 MemberPickerSheet） -->
    <MemberPickerSheet
      :open="memberPickerDraft !== null"
      :model-value="memberPickerDraft?.member_id || myMember?.user_id || ''"
      :members="store.activeBook?.members || []"
      @update:open="(v: boolean) => { if (!v) memberPickerDraft = null; }"
      @update:model-value="onSelectDraftMember"
    />
</template>
