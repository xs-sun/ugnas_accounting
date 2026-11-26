<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { todayIso } from '@/utils/format';

// 移动端日期时间选择器：底部弹出层，上半是月历（点选日期），
// 下半是 iOS 风格的滚轮（时/分两列，滚动吸附居中），底部取消/确定。
// 解决原生 <input type="date/time"> 在手机上又丑又难用的问题；
// 打开时用传入值做草稿，取消不回写，只有确定才提交。
export interface DateTimeValue {
  date: string; // YYYY-MM-DD
  time: string; // HH:mm
}

const props = withDefaults(
  defineProps<{
    open: boolean;
    modelValue: DateTimeValue;
    /** datetime：月历 + 时间滚轮；date：只要月历（比如定时规则的开始日期） */
    mode?: 'datetime' | 'date';
  }>(),
  { mode: 'datetime' }
);

const emit = defineEmits<{
  'update:modelValue': [val: DateTimeValue];
  'update:open': [open: boolean];
  close: [];
}>();

const ITEM_H = 40; // 滚轮每行高度（px）

const draftDate = ref(props.modelValue.date || todayIso());
const draftHour = ref(0);
const draftMinute = ref(0);
const calYear = ref(0);
const calMonth = ref(0); // 0-11

const hourCol = ref<HTMLElement | null>(null);
const minuteCol = ref<HTMLElement | null>(null);
let hourTimer: number | undefined;
let minuteTimer: number | undefined;

function parseTime(t: string) {
  const [h, m] = (t || '09:00').split(':');
  return {
    hour: Math.min(23, Math.max(0, Number(h) || 0)),
    minute: Math.min(59, Math.max(0, Number(m) || 0)),
  };
}

function initDraft() {
  draftDate.value = props.modelValue.date || todayIso();
  const { hour, minute } = parseTime(props.modelValue.time);
  draftHour.value = hour;
  draftMinute.value = minute;
  const d = new Date(`${draftDate.value}T12:00:00`);
  if (!Number.isNaN(d.getTime())) {
    calYear.value = d.getFullYear();
    calMonth.value = d.getMonth();
  } else {
    const now = new Date();
    calYear.value = now.getFullYear();
    calMonth.value = now.getMonth();
  }
  void nextTick(() => {
    scrollColTo(hourCol.value, draftHour.value, false);
    scrollColTo(minuteCol.value, draftMinute.value, false);
  });
}

watch(
  () => props.open,
  (opened) => {
    if (opened) initDraft();
  },
  { immediate: true }
);

function close() {
  emit('update:open', false);
  emit('close');
}

function confirm() {
  emit('update:modelValue', {
    date: draftDate.value,
    time:
      props.mode === 'date'
        ? props.modelValue.time
        : `${String(draftHour.value).padStart(2, '0')}:${String(draftMinute.value).padStart(2, '0')}`,
  });
  close();
}

function shiftCalMonth(delta: number) {
  const m = calMonth.value + delta;
  calYear.value = calYear.value + Math.floor(m / 12);
  calMonth.value = ((m % 12) + 12) % 12;
}

function goToday() {
  const iso = todayIso();
  draftDate.value = iso;
  const d = new Date();
  calYear.value = d.getFullYear();
  calMonth.value = d.getMonth();
}

const calCells = computed(() => {
  const first = new Date(calYear.value, calMonth.value, 1);
  const startWeekday = (first.getDay() + 6) % 7; // 周一起头
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

const todayStr = computed(() => todayIso());

function selectDay(dateStr: string) {
  draftDate.value = dateStr;
}

const draftLabel = computed(() => {
  const [y, m, d] = draftDate.value.split('-').map(Number);
  const datePart = `${y}年${m}月${d}日`;
  if (props.mode === 'date') return datePart;
  return `${datePart} ${String(draftHour.value).padStart(2, '0')}:${String(draftMinute.value).padStart(2, '0')}`;
});

const hours = Array.from({ length: 24 }, (_, i) => i);
const minutes = Array.from({ length: 60 }, (_, i) => i);

function scrollColTo(el: HTMLElement | null, index: number, smooth = true) {
  if (!el) return;
  el.scrollTo({ top: index * ITEM_H, behavior: smooth ? 'smooth' : 'auto' });
}

// 滚轮停下后吸附到最近一行并回写草稿时间；用 scrollend + 定时器兜底
//（部分 WebView 不触发 scrollend）。
function onWheelScroll(kind: 'hour' | 'minute') {
  const el = kind === 'hour' ? hourCol.value : minuteCol.value;
  if (!el) return;
  const timer = kind === 'hour' ? hourTimer : minuteTimer;
  if (timer !== undefined) window.clearTimeout(timer);
  const t = window.setTimeout(() => settleWheel(kind), 90);
  if (kind === 'hour') hourTimer = t;
  else minuteTimer = t;
}

function settleWheel(kind: 'hour' | 'minute') {
  const el = kind === 'hour' ? hourCol.value : minuteCol.value;
  if (!el) return;
  const max = kind === 'hour' ? 23 : 59;
  const idx = Math.min(max, Math.max(0, Math.round(el.scrollTop / ITEM_H)));
  if (kind === 'hour') draftHour.value = idx;
  else draftMinute.value = idx;
  if (Math.abs(el.scrollTop - idx * ITEM_H) > 1) {
    el.scrollTo({ top: idx * ITEM_H, behavior: 'smooth' });
  }
}

function tapWheelItem(kind: 'hour' | 'minute', index: number) {
  if (kind === 'hour') draftHour.value = index;
  else draftMinute.value = index;
  scrollColTo(kind === 'hour' ? hourCol.value : minuteCol.value, index);
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[110] flex items-end justify-center">
      <div class="absolute inset-0 bg-[#14171A]/50" @click="close" />
      <div
        class="relative w-full max-w-md bg-white rounded-t-3xl px-4 pt-2 pb-5 max-h-[82vh] overflow-y-auto phone-scroll"
      >
        <div class="w-10 h-1 rounded-full bg-[#E6E9EF] mx-auto my-2" />
        <div class="flex items-center justify-between px-1 mb-1">
          <button
            type="button"
            class="text-sm text-[#64748B] py-1"
            @click="close"
          >
            取消
          </button>
          <p class="text-sm font-bold text-[#18181B]">{{ mode === 'date' ? '选择日期' : '选择日期时间' }}</p>
          <button
            type="button"
            class="text-sm font-semibold text-[#DE8A1E] py-1"
            @click="confirm"
          >
            确定
          </button>
        </div>
        <p class="text-center text-xs text-[#64748B] mb-3">{{ draftLabel }}</p>

        <!-- 月历 -->
        <div class="flex items-center justify-between px-1 mb-2">
          <button
            type="button"
            class="w-8 h-8 rounded-full bg-[#F5F6F8] text-[#475569] text-base leading-none"
            @click="shiftCalMonth(-1)"
            aria-label="上个月"
          >
            ‹
          </button>
          <span class="text-sm font-semibold text-[#18181B]">
            {{ calYear }}年{{ calMonth + 1 }}月
          </span>
          <button
            type="button"
            class="w-8 h-8 rounded-full bg-[#F5F6F8] text-[#475569] text-base leading-none"
            @click="shiftCalMonth(1)"
            aria-label="下个月"
          >
            ›
          </button>
        </div>
        <div class="grid grid-cols-7 text-center text-[11px] text-[#94A3B8] mb-1">
          <span v-for="w in ['一', '二', '三', '四', '五', '六', '日']" :key="w">{{ w }}</span>
        </div>
        <div class="grid grid-cols-7 gap-y-1">
          <button
            v-for="(cell, i) in calCells"
            :key="i"
            type="button"
            :disabled="!cell.inMonth"
            class="h-9 flex items-center justify-center"
            @click="cell.inMonth && selectDay(cell.dateStr)"
          >
            <span
              v-if="cell.inMonth"
              class="w-9 h-9 flex items-center justify-center rounded-full text-sm transition"
              :class="
                cell.dateStr === draftDate
                  ? 'bg-[#18181B] text-white font-semibold'
                  : cell.dateStr === todayStr
                    ? 'text-[#DE8A1E] font-semibold'
                    : 'text-[#18181B]'
              "
            >
              {{ cell.day }}
            </span>
          </button>
        </div>
        <button
          type="button"
          class="mt-1 text-xs font-medium text-[#DE8A1E] px-1 py-1"
          @click="goToday"
        >
          回到今天
        </button>

        <!-- 时间滚轮（date 模式不要） -->
        <div v-if="mode === 'datetime'" class="mt-2 pt-3 border-t border-[#F1F3F6]">
          <p class="text-xs font-semibold text-[#64748B] px-1 mb-1">时间</p>
          <div class="relative h-[200px] overflow-hidden">
            <div
              class="absolute inset-x-0 top-1/2 -translate-y-1/2 h-10 rounded-xl bg-[#F5F6F8] pointer-events-none"
            />
            <div
              class="absolute inset-x-0 top-0 h-[80px] pointer-events-none z-10"
              style="
                background: linear-gradient(to bottom, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0));
              "
            />
            <div
              class="absolute inset-x-0 bottom-0 h-[80px] pointer-events-none z-10"
              style="
                background: linear-gradient(to top, rgba(255, 255, 255, 0.95), rgba(255, 255, 255, 0));
              "
            />
            <div class="h-full flex items-stretch justify-center gap-2">
              <div
                ref="hourCol"
                class="w-20 h-full overflow-y-auto phone-scroll snap-y snap-mandatory"
                style="scrollbar-width: none"
                @scroll="onWheelScroll('hour')"
                @scrollend="settleWheel('hour')"
              >
                <div :style="{ paddingTop: '80px', paddingBottom: '80px' }">
                  <button
                    v-for="h in hours"
                    :key="h"
                    type="button"
                    class="w-full h-10 snap-center flex items-center justify-center text-base transition"
                    :class="h === draftHour ? 'font-bold text-[#18181B]' : 'text-[#94A3B8]'"
                    @click="tapWheelItem('hour', h)"
                  >
                    {{ String(h).padStart(2, '0') }}
                  </button>
                </div>
              </div>
              <div class="h-full flex items-center text-lg font-bold text-[#18181B]">:</div>
              <div
                ref="minuteCol"
                class="w-20 h-full overflow-y-auto phone-scroll snap-y snap-mandatory"
                style="scrollbar-width: none"
                @scroll="onWheelScroll('minute')"
                @scrollend="settleWheel('minute')"
              >
                <div :style="{ paddingTop: '80px', paddingBottom: '80px' }">
                  <button
                    v-for="m in minutes"
                    :key="m"
                    type="button"
                    class="w-full h-10 snap-center flex items-center justify-center text-base transition"
                    :class="m === draftMinute ? 'font-bold text-[#18181B]' : 'text-[#94A3B8]'"
                    @click="tapWheelItem('minute', m)"
                  >
                    {{ String(m).padStart(2, '0') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <button
          type="button"
          class="mt-4 w-full h-11 rounded-full bg-[#DE8A1E] text-white text-sm font-semibold"
          @click="confirm"
        >
          确定
        </button>
      </div>
    </div>
  </Teleport>
</template>
