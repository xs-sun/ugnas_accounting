<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue';
import { CAT_AVATARS, resolveAvatarUrl } from '@/constants/avatars';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';

const props = withDefaults(
  defineProps<{
    open?: boolean;
    currentAvatar?: string;
    target?: string;
  }>(),
  {
    open: undefined,
    currentAvatar: '',
    target: '',
  }
);

const emit = defineEmits<{
  close: [];
  select: [avatar: string];
}>();

const store = useAppStore();

const isControlled = computed(() => props.open !== undefined);
const isOpen = computed(() =>
  isControlled.value ? Boolean(props.open) : store.avatarModal.open
);

const effectiveTarget = computed(
  () => props.target || store.avatarModal.target || 'profile'
);

const effectiveCurrentAvatar = computed(() => {
  const raw =
    props.currentAvatar ||
    store.avatarModal.currentAvatar ||
    store.user?.avatar ||
    '';
  return resolveAvatarUrl(raw);
});

const saving = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);
const cropCanvasRef = ref<HTMLCanvasElement | null>(null);

const cropper = reactive<{
  active: boolean;
  img: HTMLImageElement | null;
  scale: number;
  offsetX: number;
  offsetY: number;
  rotation: number;
}>({
  active: false,
  img: null,
  scale: 1,
  offsetX: 0,
  offsetY: 0,
  rotation: 0,
});

let dragging = false;
let dragStartX = 0;
let dragStartY = 0;
let baseOffsetX = 0;
let baseOffsetY = 0;

watch(isOpen, (val) => {
  if (val) {
    resetCropper();
  }
});

function resetCropper() {
  cropper.active = false;
  cropper.img = null;
  cropper.scale = 1;
  cropper.offsetX = 0;
  cropper.offsetY = 0;
  cropper.rotation = 0;
}

function handleClose() {
  resetCropper();
  if (!isControlled.value) {
    store.closeAvatarModal();
  }
  emit('close');
}

async function applyChosenAvatar(avatarValue: string) {
  if (saving.value) return;
  saving.value = true;
  try {
    if (store.avatarModal.onSelect) {
      store.avatarModal.onSelect(avatarValue);
    } else if (
      !isControlled.value &&
      effectiveTarget.value === 'profile' &&
      store.user
    ) {
      try {
        const updated = await api.updateProfile({ avatar: avatarValue });
        store.user = updated;
      } catch {
        // 离线或访客模式下至少更新本地视图状态
        store.user = { ...store.user, avatar: avatarValue };
      }
    }
    emit('select', avatarValue);
    store.showToast('头像已更新', 'success');
    handleClose();
  } finally {
    saving.value = false;
  }
}

function pickRandomCat() {
  const curResolved = effectiveCurrentAvatar.value;
  const pool = CAT_AVATARS.filter((c) => c.src !== curResolved);
  const picked =
    (pool.length > 0 ? pool : CAT_AVATARS)[
      Math.floor(Math.random() * (pool.length > 0 ? pool.length : CAT_AVATARS.length))
    ];
  applyChosenAvatar(picked.publicPath);
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = () => {
    const img = new Image();
    img.onload = async () => {
      cropper.active = true;
      cropper.img = img;
      cropper.scale = 1;
      cropper.offsetX = 0;
      cropper.offsetY = 0;
      cropper.rotation = 0;
      await nextTick();
      drawCropCanvas();
    };
    img.src = String(reader.result || '');
  };
  reader.readAsDataURL(file);
  input.value = '';
}

function drawCropCanvas() {
  const canvas = cropCanvasRef.value;
  if (!canvas || !cropper.img) return;
  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  const size = canvas.width;
  ctx.clearRect(0, 0, size, size);

  const baseScale = Math.max(size / cropper.img.width, size / cropper.img.height);
  const finalScale = baseScale * cropper.scale;

  ctx.save();
  ctx.translate(size / 2 + cropper.offsetX, size / 2 + cropper.offsetY);
  ctx.rotate((cropper.rotation * Math.PI) / 180);
  ctx.scale(finalScale, finalScale);
  ctx.drawImage(cropper.img, -cropper.img.width / 2, -cropper.img.height / 2);
  ctx.restore();

  // 圆形裁切遮罩与描边
  ctx.save();
  ctx.fillStyle = 'rgba(15, 23, 42, 0.48)';
  ctx.beginPath();
  ctx.rect(0, 0, size, size);
  ctx.arc(size / 2, size / 2, size * 0.44, 0, Math.PI * 2, true);
  ctx.fill('evenodd');
  ctx.strokeStyle = '#FFFFFF';
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.arc(size / 2, size / 2, size * 0.44, 0, Math.PI * 2);
  ctx.stroke();
  ctx.restore();
}

function exportCroppedAvatar(): string | null {
  if (!cropper.img) return null;
  const out = document.createElement('canvas');
  const outSize = 256;
  out.width = outSize;
  out.height = outSize;
  const ctx = out.getContext('2d');
  if (!ctx) return null;

  const previewSize = 220;
  const cropRadius = previewSize * 0.44;
  const ratio = outSize / (cropRadius * 2);
  const baseScale = Math.max(
    previewSize / cropper.img.width,
    previewSize / cropper.img.height
  );
  const finalScale = baseScale * cropper.scale * ratio;

  ctx.save();
  ctx.beginPath();
  ctx.arc(outSize / 2, outSize / 2, outSize / 2, 0, Math.PI * 2);
  ctx.closePath();
  ctx.clip();
  ctx.fillStyle = '#FFFFFF';
  ctx.fillRect(0, 0, outSize, outSize);
  ctx.translate(
    outSize / 2 + cropper.offsetX * ratio,
    outSize / 2 + cropper.offsetY * ratio
  );
  ctx.rotate((cropper.rotation * Math.PI) / 180);
  ctx.scale(finalScale, finalScale);
  ctx.drawImage(cropper.img, -cropper.img.width / 2, -cropper.img.height / 2);
  ctx.restore();

  return out.toDataURL('image/jpeg', 0.9);
}

function onPointerDown(e: PointerEvent) {
  dragging = true;
  dragStartX = e.clientX;
  dragStartY = e.clientY;
  baseOffsetX = cropper.offsetX;
  baseOffsetY = cropper.offsetY;
  const canvas = cropCanvasRef.value;
  if (canvas?.setPointerCapture) {
    try {
      canvas.setPointerCapture(e.pointerId);
    } catch {
      // ignore
    }
  }
}

function onPointerMove(e: PointerEvent) {
  if (!dragging) return;
  cropper.offsetX = Math.max(-140, Math.min(140, baseOffsetX + (e.clientX - dragStartX)));
  cropper.offsetY = Math.max(-140, Math.min(140, baseOffsetY + (e.clientY - dragStartY)));
  drawCropCanvas();
}

function onPointerUp() {
  dragging = false;
}

function onZoomInput(e: Event) {
  const val = parseFloat((e.target as HTMLInputElement).value);
  cropper.scale = Number.isFinite(val) ? val : 1;
  drawCropCanvas();
}

function rotate90() {
  cropper.rotation = (cropper.rotation + 90) % 360;
  drawCropCanvas();
}

function confirmCrop() {
  const dataUrl = exportCroppedAvatar();
  if (dataUrl) {
    applyChosenAvatar(dataUrl);
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isOpen"
      class="fixed inset-0 z-[80] flex items-end sm:items-center justify-center sm:p-4"
    >
      <div
        class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
        @click="handleClose"
      />

      <div
        class="relative w-full sm:max-w-[440px] max-h-[88vh] bg-[#F5F6F8] rounded-t-[28px] sm:rounded-[26px] border border-white/80 shadow-2xl flex flex-col overflow-hidden"
      >
        <!-- Header -->
        <div class="px-5 py-3.5 bg-white border-b border-[#ECEEF2] flex items-center justify-between shrink-0">
          <h3 class="text-[15px] font-bold text-[#18181B]">
            {{ cropper.active ? '拖动与缩放裁剪头像' : '选择或上传头像' }}
          </h3>
          <button
            type="button"
            class="w-8 h-8 rounded-full bg-[#F5F6F8] hover:bg-[#ECEEF2] flex items-center justify-center text-[#64748B] hover:text-[#18181B] transition"
            @click="handleClose"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Body -->
        <div class="p-4 overflow-y-auto thin-scroll space-y-3.5">
          <!-- Preset Gallery Mode -->
          <template v-if="!cropper.active">
            <div class="app-card p-4 flex items-center justify-between gap-3">
              <div class="flex items-center gap-3 min-w-0">
                <img
                  :src="effectiveCurrentAvatar"
                  alt="当前头像"
                  class="w-14 h-14 rounded-full object-cover bg-white border border-[#ECEEF2] shadow-xs shrink-0"
                />
                <div class="min-w-0">
                  <p class="text-xs font-bold text-[#18181B]">七猫默认头像库</p>
                  <p class="text-[11px] text-[#64748B] mt-0.5 truncate">
                    选一只可爱小猫，或上传相册裁剪
                  </p>
                </div>
              </div>
              <button
                type="button"
                :disabled="saving"
                class="shrink-0 text-[11px] font-semibold px-3 py-1.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition"
                @click="pickRandomCat"
              >
                随机一只
              </button>
            </div>

            <div class="app-card p-3.5">
              <div class="grid grid-cols-3 gap-3">
                <button
                  v-for="cat in CAT_AVATARS"
                  :key="cat.id"
                  type="button"
                  :disabled="saving"
                  class="flex flex-col items-center gap-1.5 p-2 rounded-2xl transition"
                  :class="
                    effectiveCurrentAvatar === cat.src
                      ? 'bg-[#FFF6E8] ring-1.5 ring-[#DE8A1E]'
                      : 'hover:bg-[#F5F6F8]'
                  "
                  @click="applyChosenAvatar(cat.publicPath)"
                >
                  <img
                    :src="cat.src"
                    :alt="cat.name"
                    class="w-16 h-16 rounded-full object-cover bg-white shadow-sm ring-1 ring-[#ECEEF2]"
                  />
                  <span
                    class="text-[11px]"
                    :class="
                      effectiveCurrentAvatar === cat.src
                        ? 'font-bold text-[#DE8A1E]'
                        : 'font-medium text-[#64748B]'
                    "
                  >
                    {{ cat.name }}
                  </span>
                </button>
              </div>
            </div>

            <label
              class="cursor-pointer w-full py-3 rounded-2xl bg-[#18181B] text-white text-xs font-semibold flex items-center justify-center gap-2 hover:bg-[#27272A] transition shadow-sm"
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
              <span>从相册上传图片并裁剪</span>
              <input
                ref="fileInputRef"
                type="file"
                accept="image/*"
                class="hidden"
                @change="onFileChange"
              />
            </label>
          </template>

          <!-- Canvas Cropper Mode -->
          <template v-else>
            <div class="app-card p-4 flex flex-col items-center">
              <canvas
                ref="cropCanvasRef"
                width="220"
                height="220"
                class="w-[220px] h-[220px] rounded-2xl bg-[#18181B] cursor-grab active:cursor-grabbing touch-none shadow-inner"
                @pointerdown="onPointerDown"
                @pointermove="onPointerMove"
                @pointerup="onPointerUp"
                @pointercancel="onPointerUp"
              />
              <p class="text-[11px] text-[#64748B] mt-2.5">
                按住图片拖动位置，拖动下方滑块缩放
              </p>

              <div class="w-full mt-3 pt-3 border-t border-[#F1F3F6] flex items-center gap-3">
                <span class="text-[11px] font-semibold text-[#64748B] shrink-0">缩放</span>
                <input
                  type="range"
                  min="1"
                  max="3"
                  step="0.05"
                  :value="cropper.scale"
                  class="flex-1 accent-[#DE8A1E]"
                  @input="onZoomInput"
                />
                <button
                  type="button"
                  class="shrink-0 text-[11px] font-semibold px-2.5 py-1 rounded-xl bg-[#F5F6F8] text-[#475569] hover:bg-[#ECEEF2] transition"
                  @click="rotate90"
                >
                  旋转90°
                </button>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-2.5">
              <button
                type="button"
                class="py-3 rounded-2xl bg-white border border-[#E6E9EF] text-xs font-semibold text-[#475569] hover:bg-[#F8F9FB] transition"
                @click="resetCropper"
              >
                重新选择
              </button>
              <button
                type="button"
                :disabled="saving"
                class="py-3 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] disabled:opacity-60 transition"
                @click="confirmCrop"
              >
                {{ saving ? '保存中…' : '确认裁剪并使用' }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>
