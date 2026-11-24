/**
 * UGOS Pro 桌面内嵌窗口（CloudWindow）默认尺寸与自适应放大工具。
 * 用于解决以 open_type: "inner" 打开时系统默认窗口偏小的问题。
 */

export const UGOS_DESKTOP_WINDOW_DEFAULTS = {
  width: 1360,
  height: 860,
  minWidth: 960,
  minHeight: 640,
} as const;

const MIN_DESKTOP_HOST_WIDTH = 900;
const HOST_MARGIN_X = 40;
const HOST_MARGIN_Y = 80;
const SDK_CALL_TIMEOUT_MS = 1500;

export interface WindowSizeInfo {
  width: number;
  height: number;
  minWidth?: number;
  minHeight?: number;
  maxWidth?: number;
  maxHeight?: number;
}

export interface HostViewportBounds {
  hostWidth: number;
  hostHeight: number;
}

export interface TargetWindowPlan {
  shouldResize: boolean;
  shouldUpdateMinSize: boolean;
  targetWidth: number;
  targetHeight: number;
  minWidth: number;
  minHeight: number;
}

export interface CloudWindowLike {
  winId?: string;
  isMaximized?: () => Promise<boolean>;
  getSizeInfo?: () => Promise<WindowSizeInfo>;
  getSize?: () => Promise<{ width: number; height: number }>;
  setMinSize?: (width: number, height: number) => Promise<void>;
  setSize?: (width: number, height: number) => Promise<void>;
  center?: () => Promise<void>;
}

let desktopWindowConfigured = false;

/**
 * 仅供单元测试重置内部幂等状态使用。
 */
export function resetUGOSDesktopWindowState(): void {
  desktopWindowConfigured = false;
}

function isMobileEnvironment(isMobileFlag?: boolean): boolean {
  if (isMobileFlag) return true;
  if (typeof navigator === 'undefined') return false;
  return /(Android|iPhone|iPod|Mobile|OpenHarmony.*Phone)/i.test(navigator.userAgent || '');
}

/**
 * 获取当前宿主桌面可用视口尺寸，同源优先读取 top 窗口，跨域回退 screen.availWidth/Height。
 */
export function resolveHostViewportBounds(): HostViewportBounds {
  if (typeof window === 'undefined') {
    return { hostWidth: 0, hostHeight: 0 };
  }

  let hostWidth = 0;
  let hostHeight = 0;

  try {
    if (
      window.top &&
      window.top !== window.self &&
      window.top.innerWidth > 0 &&
      window.top.innerHeight > 0
    ) {
      hostWidth = window.top.innerWidth;
      hostHeight = window.top.innerHeight;
    }
  } catch {
    // 跨域环境下无法读取 window.top，静默回退到屏幕可用区域
  }

  if (!hostWidth || !hostHeight) {
    hostWidth = window.screen?.availWidth || window.innerWidth || 0;
    hostHeight = window.screen?.availHeight || window.innerHeight || 0;
  }

  return { hostWidth, hostHeight };
}

/**
 * 根据当前窗口几何信息与宿主可用视口，计算安全的目标窗口调整方案。
 */
export function computeTargetWindowPlan(
  current: WindowSizeInfo,
  host: HostViewportBounds
): TargetWindowPlan | null {
  if (current.width <= 0 || current.height <= 0) return null;
  if (host.hostWidth <= 0 || host.hostHeight <= 0) return null;
  if (host.hostWidth < MIN_DESKTOP_HOST_WIDTH) return null;

  const usableW = Math.max(320, host.hostWidth - HOST_MARGIN_X);
  const usableH = Math.max(320, host.hostHeight - HOST_MARGIN_Y);

  const capW =
    current.maxWidth && current.maxWidth > 0 ? Math.min(usableW, current.maxWidth) : usableW;
  const capH =
    current.maxHeight && current.maxHeight > 0 ? Math.min(usableH, current.maxHeight) : usableH;

  const desiredW = Math.round(Math.min(UGOS_DESKTOP_WINDOW_DEFAULTS.width, capW));
  const desiredH = Math.round(Math.min(UGOS_DESKTOP_WINDOW_DEFAULTS.height, capH));

  const currentW = Math.round(current.width);
  const currentH = Math.round(current.height);

  // 只放大偏小的默认窗口，绝不把已更大的窗口缩回去
  const targetWidth = Math.max(currentW, desiredW);
  const targetHeight = Math.max(currentH, desiredH);

  const minWidth = Math.round(Math.min(UGOS_DESKTOP_WINDOW_DEFAULTS.minWidth, targetWidth));
  const minHeight = Math.round(Math.min(UGOS_DESKTOP_WINDOW_DEFAULTS.minHeight, targetHeight));

  const shouldResize = targetWidth > currentW || targetHeight > currentH;
  const curMinW = current.minWidth ?? 0;
  const curMinH = current.minHeight ?? 0;
  const shouldUpdateMinSize = curMinW < minWidth || curMinH < minHeight;

  return {
    shouldResize,
    shouldUpdateMinSize,
    targetWidth,
    targetHeight,
    minWidth,
    minHeight,
  };
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | null = null;
  return Promise.race([
    promise.finally(() => {
      if (timer) clearTimeout(timer);
    }),
    new Promise<T>((_, reject) => {
      timer = setTimeout(() => reject(new Error('UGOS window operation timeout')), timeoutMs);
    }),
  ]);
}

async function readCurrentWindowSize(cloudWindow: CloudWindowLike): Promise<WindowSizeInfo | null> {
  if (typeof cloudWindow.getSizeInfo === 'function') {
    return withTimeout(cloudWindow.getSizeInfo(), SDK_CALL_TIMEOUT_MS);
  }
  if (typeof cloudWindow.getSize === 'function') {
    return withTimeout(cloudWindow.getSize(), SDK_CALL_TIMEOUT_MS);
  }
  return null;
}

/**
 * 在应用首次嵌入 UGOS Pro PC 桌面窗口打开时，若窗口偏小则自动调整为舒适宽度并居中。
 */
export async function ensureUGOSDesktopWindowSize(
  cloudWindow: CloudWindowLike | null | undefined,
  options?: { isMobile?: boolean; hostBounds?: HostViewportBounds }
): Promise<boolean> {
  if (desktopWindowConfigured || !cloudWindow || !cloudWindow.winId) {
    return false;
  }
  if (isMobileEnvironment(options?.isMobile)) {
    desktopWindowConfigured = true;
    return false;
  }

  // 立即标记已执行初始化尝试，避免后续重渲染或用户手动拖拽后被覆盖
  desktopWindowConfigured = true;

  try {
    if (typeof cloudWindow.isMaximized === 'function') {
      const maximized = await withTimeout(cloudWindow.isMaximized(), SDK_CALL_TIMEOUT_MS);
      if (maximized) return false;
    }

    const sizeInfo = await readCurrentWindowSize(cloudWindow);
    if (!sizeInfo) return false;

    const hostBounds = options?.hostBounds ?? resolveHostViewportBounds();
    const plan = computeTargetWindowPlan(sizeInfo, hostBounds);
    if (!plan) return false;

    // 先设置最小宽高限制，再设置新尺寸；串行 await 防止 setMinSize 内部 getConfig 回写旧尺寸
    if (plan.shouldUpdateMinSize && typeof cloudWindow.setMinSize === 'function') {
      await withTimeout(
        cloudWindow.setMinSize(plan.minWidth, plan.minHeight),
        SDK_CALL_TIMEOUT_MS
      );
    }

    if (plan.shouldResize && typeof cloudWindow.setSize === 'function') {
      await withTimeout(
        cloudWindow.setSize(plan.targetWidth, plan.targetHeight),
        SDK_CALL_TIMEOUT_MS
      );
      if (typeof cloudWindow.center === 'function') {
        await withTimeout(cloudWindow.center(), SDK_CALL_TIMEOUT_MS);
      }
      return true;
    }

    return false;
  } catch {
    // 非 UGOS 宿主或低版本跨域环境下静默忽略，不影响应用正常启动
    return false;
  }
}
