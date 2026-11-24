// 双层双色 SVG 图标库（完整移植自设计稿）
export const ICON_LIBRARY: Record<string, string> = {
  bowl: '<path d="M4.5 12.5h15a7.5 7.5 0 0 1-15 0z" fill="currentColor" fill-opacity="0.18"/><path d="M4.5 12.5h15a7.5 7.5 0 0 1-15 0z"/><path d="M8.5 19.5h7"/><path d="M9 8.5c0-1.1.9-1.5.9-2.7M12 8.5c0-1.1.9-1.5.9-2.7M15 8.5c0-1.1.9-1.5.9-2.7"/>',
  coffee: '<path d="M5.5 9h10.5v6.2a4.3 4.3 0 0 1-4.3 4.3H9.8a4.3 4.3 0 0 1-4.3-4.3V9z" fill="currentColor" fill-opacity="0.18"/><path d="M5.5 9h10.5v6.2a4.3 4.3 0 0 1-4.3 4.3H9.8a4.3 4.3 0 0 1-4.3-4.3V9z"/><path d="M16 10.5h1.8a2.2 2.2 0 0 1 0 4.4H16"/><path d="M8.5 5.8c0-.9.7-1.2.7-2M12.2 5.8c0-.9.7-1.2.7-2"/>',
  breakfast: '<rect x="5" y="7" width="14" height="12" rx="3.5" fill="currentColor" fill-opacity="0.16"/><path d="M7.5 7a3.5 3.5 0 0 1 9 0"/><rect x="5" y="7" width="14" height="12" rx="3.5"/><path d="M9 12.5h6M9 15.5h4"/>',
  bento: '<rect x="4" y="6" width="16" height="12" rx="3" fill="currentColor" fill-opacity="0.16"/><rect x="4" y="6" width="16" height="12" rx="3"/><path d="M11 6v12M11 12h9"/><circle cx="7.5" cy="12" r="1.4" fill="currentColor" fill-opacity="0.4"/>',
  fruit: '<path d="M12 8c-3.5-1.5-6.5 1.2-6.5 5.2 0 3.6 2.5 6.8 5 6.8 1 0 1.2-.5 1.5-.5s.5.5 1.5.5c2.5 0 5-3.2 5-6.8C18.5 9.2 15.5 6.5 12 8z" fill="currentColor" fill-opacity="0.18"/><path d="M12 8c-3.5-1.5-6.5 1.2-6.5 5.2 0 3.6 2.5 6.8 5 6.8 1 0 1.2-.5 1.5-.5s.5.5 1.5.5c2.5 0 5-3.2 5-6.8C18.5 9.2 15.5 6.5 12 8z"/><path d="M12 8c0-2 1.2-3.5 3-4"/>',
  dining: '<path d="M7 4v5.5a2.5 2.5 0 0 0 5 0V4M9.5 4v16" /><path d="M15.5 4c2 0 3 2.2 3 5.5v2h-3V4z" fill="currentColor" fill-opacity="0.18"/><path d="M15.5 4c2 0 3 2.2 3 5.5v2h-3V4zm0 7.5V20"/>',
  bag: '<path d="M6.2 8.5h11.6l-1 10.2a2 2 0 0 1-2 1.8H9.2a2 2 0 0 1-2-1.8l-1-10.2z" fill="currentColor" fill-opacity="0.18"/><path d="M6.2 8.5h11.6l-1 10.2a2 2 0 0 1-2 1.8H9.2a2 2 0 0 1-2-1.8l-1-10.2z"/><path d="M9.5 8.5V6.6a2.5 2.5 0 0 1 5 0v1.9"/><path d="M9.5 12.8h5"/>',
  cart: '<path d="M6.5 8h13l-1.5 7.5H8L6.5 8z" fill="currentColor" fill-opacity="0.18"/><path d="M3.5 5h2.2l2.3 10.5h9.8l1.5-7.5H6.5"/><circle cx="9.5" cy="19" r="1.4" fill="currentColor"/><circle cx="16.5" cy="19" r="1.4" fill="currentColor"/>',
  shirt: '<path d="M8.5 5.5L12 7.5l3.5-2 4 3-2 3-1.5-1V19.5H8V10.5l-1.5 1-2-3 4-3z" fill="currentColor" fill-opacity="0.16"/><path d="M8.5 5.5L12 7.5l3.5-2 4 3-2 3-1.5-1V19.5H8V10.5l-1.5 1-2-3 4-3z"/>',
  laptop: '<rect x="4.5" y="5.5" width="15" height="10.5" rx="2" fill="currentColor" fill-opacity="0.16"/><rect x="4.5" y="5.5" width="15" height="10.5" rx="2"/><path d="M2.8 18.5h18.4"/>',
  sparkles: '<path d="M12 4l1.7 5.3L19 11l-5.3 1.7L12 18l-1.7-5.3L5 11l5.3-1.7L12 4z" fill="currentColor" fill-opacity="0.18"/><path d="M12 4l1.7 5.3L19 11l-5.3 1.7L12 18l-1.7-5.3L5 11l5.3-1.7L12 4z"/><path d="M18.5 4.5v2.5M17.2 5.7h2.5"/>',
  train: '<rect x="5.5" y="4.5" width="13" height="12.5" rx="3.5" fill="currentColor" fill-opacity="0.16"/><rect x="5.5" y="4.5" width="13" height="12.5" rx="3.5"/><path d="M5.5 11h13M8.5 17l-1.5 2.5M15.5 17l1.5 2.5"/><circle cx="8.8" cy="14" r="1" fill="currentColor"/><circle cx="15.2" cy="14" r="1" fill="currentColor"/>',
  car: '<path d="M5 11.5l1.6-4.2A2 2 0 0 1 8.5 6h7a2 2 0 0 1 1.9 1.3l1.6 4.2v5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 16.5v-5z" fill="currentColor" fill-opacity="0.16"/><path d="M5 11.5l1.6-4.2A2 2 0 0 1 8.5 6h7a2 2 0 0 1 1.9 1.3l1.6 4.2v5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 16.5v-5z"/><path d="M5 11.5h14M7.5 18v1.5M16.5 18v1.5"/><circle cx="8" cy="14.5" r="1" fill="currentColor"/><circle cx="16" cy="14.5" r="1" fill="currentColor"/>',
  fuel: '<rect x="4.5" y="5" width="10" height="14.5" rx="2" fill="currentColor" fill-opacity="0.16"/><rect x="4.5" y="5" width="10" height="14.5" rx="2"/><path d="M7 9.5h5M14.5 10h2a2 2 0 0 1 2 2v4a1.5 1.5 0 0 0 1.5 1.5"/>',
  plane: '<path d="M12 4l2.2 6.5 5.8 2v2l-5.8-1.2V17l2 1.5V20l-4.2-1-4.2 1v-1.5l2-1.5v-3.7L4 14.5v-2l5.8-2L12 4z" fill="currentColor" fill-opacity="0.18"/><path d="M12 4l2.2 6.5 5.8 2v2l-5.8-1.2V17l2 1.5V20l-4.2-1-4.2 1v-1.5l2-1.5v-3.7L4 14.5v-2l5.8-2L12 4z"/>',
  home: '<path d="M4.5 11L12 4.8 19.5 11v8.2a1.3 1.3 0 0 1-1.3 1.3H5.8a1.3 1.3 0 0 1-1.3-1.3V11z" fill="currentColor" fill-opacity="0.16"/><path d="M3.5 11.2L12 4.5l8.5 6.7"/><path d="M5.5 10v9a1.5 1.5 0 0 0 1.5 1.5h10a1.5 1.5 0 0 0 1.5-1.5v-9"/><path d="M10 20.5v-5.5a2 2 0 0 1 4 0v5.5"/>',
  bolt: '<path d="M13 3.5L5.8 13h5.4l-1.2 7.5L18.2 11h-5.4L13 3.5z" fill="currentColor" fill-opacity="0.18"/><path d="M13 3.5L5.8 13h5.4l-1.2 7.5L18.2 11h-5.4L13 3.5z"/>',
  wifi: '<circle cx="12" cy="17.8" r="1.5" fill="currentColor"/><path d="M8.2 14.2a5.5 5.5 0 0 1 7.6 0M5 11a10 10 0 0 1 14 0"/>',
  wrench: '<path d="M14.8 6.2a4 4 0 0 0-4.9 4.9L5 16a1.8 1.8 0 0 0 2.6 2.6l4.9-4.9a4 4 0 0 0 4.9-4.9l-2.3 2.3-2.6-2.6 2.3-2.3z" fill="currentColor" fill-opacity="0.16"/><path d="M14.8 6.2a4 4 0 0 0-4.9 4.9L5 16a1.8 1.8 0 0 0 2.6 2.6l4.9-4.9a4 4 0 0 0 4.9-4.9l-2.3 2.3-2.6-2.6 2.3-2.3z"/>',
  film: '<rect x="4" y="5.5" width="16" height="13" rx="2.5" fill="currentColor" fill-opacity="0.16"/><rect x="4" y="5.5" width="16" height="13" rx="2.5"/><path d="M8 5.5v13M16 5.5v13M4 9.5h4M4 14.5h4M16 9.5h4M16 14.5h4"/>',
  gamepad: '<rect x="3.5" y="7.5" width="17" height="9.5" rx="4.75" fill="currentColor" fill-opacity="0.17"/><rect x="3.5" y="7.5" width="17" height="9.5" rx="4.75"/><path d="M8.5 10.5v3.5M6.8 12.2h3.5"/><circle cx="15.2" cy="11.3" r="1" fill="currentColor"/><circle cx="17.2" cy="13.2" r="1" fill="currentColor"/>',
  dumbbell: '<rect x="4.5" y="8" width="3" height="8" rx="1.2" fill="currentColor" fill-opacity="0.2"/><rect x="16.5" y="8" width="3" height="8" rx="1.2" fill="currentColor" fill-opacity="0.2"/><rect x="4.5" y="8" width="3" height="8" rx="1.2"/><rect x="16.5" y="8" width="3" height="8" rx="1.2"/><path d="M7.5 12h9M3 10v4M21 10v4"/>',
  compass: '<circle cx="12" cy="12" r="8.2" fill="currentColor" fill-opacity="0.15"/><circle cx="12" cy="12" r="8.2"/><polygon points="15.2,8.8 13.4,13.4 8.8,15.2 10.6,10.6" fill="currentColor" fill-opacity="0.35" stroke="currentColor" stroke-width="1.5"/>',
  medkit: '<rect x="4.5" y="6" width="15" height="13.5" rx="3.2" fill="currentColor" fill-opacity="0.16"/><rect x="4.5" y="6" width="15" height="13.5" rx="3.2"/><path d="M9 6V4.5A1.5 1.5 0 0 1 10.5 3h3A1.5 1.5 0 0 1 15 4.5V6"/><path d="M12 9.8v5.8M9.1 12.7h5.8"/>',
  book: '<path d="M4.5 6h5.8A2.7 2.7 0 0 1 13 8.7v10.8a2 2 0 0 0-2-2H4.5V6z" fill="currentColor" fill-opacity="0.16"/><path d="M4.5 6h5.8A2.7 2.7 0 0 1 13 8.7v10.8a2 2 0 0 0-2-2H4.5V6z"/><path d="M19.5 6h-5.8A2.7 2.7 0 0 0 11 8.7v10.8a2 2 0 0 1 2-2h6.5V6z"/>',
  paw: '<path d="M7.5 16.2c0-2.5 2-4.2 4.5-4.2s4.5 1.7 4.5 4.2c0 1.6-1.3 2.8-4.5 2.8s-4.5-1.2-4.5-2.8z" fill="currentColor" fill-opacity="0.2"/><path d="M7.5 16.2c0-2.5 2-4.2 4.5-4.2s4.5 1.7 4.5 4.2c0 1.6-1.3 2.8-4.5 2.8s-4.5-1.2-4.5-2.8z"/><circle cx="7.4" cy="9.4" r="1.4" fill="currentColor"/><circle cx="12" cy="7.4" r="1.4" fill="currentColor"/><circle cx="16.6" cy="9.4" r="1.4" fill="currentColor"/>',
  gift: '<rect x="4.5" y="10" width="15" height="9.5" rx="2" fill="currentColor" fill-opacity="0.16"/><rect x="4.5" y="10" width="15" height="9.5" rx="2"/><path d="M3.5 10h17M12 6.5v13"/><path d="M12 6.5c-1.2-2-4-2-4 0s2.5 2 4 0zm0 0c1.2-2 4-2 4 0s-2.5 2-4 0z"/>',
  wallet: '<rect x="3.5" y="6" width="17" height="12.5" rx="3" fill="currentColor" fill-opacity="0.16"/><rect x="3.5" y="6" width="17" height="12.5" rx="3"/><path d="M15 10.5h5.5v3.5H15a1.75 1.75 0 0 1 0-3.5z"/><circle cx="16.5" cy="12.25" r="0.8" fill="currentColor"/>',
  star: '<path d="M12 3.8l2.5 5 5.5.8-4 3.9.9 5.5-4.9-2.6-4.9 2.6.9-5.5-4-3.9 5.5-.8L12 3.8z" fill="currentColor" fill-opacity="0.18"/><path d="M12 3.8l2.5 5 5.5.8-4 3.9.9 5.5-4.9-2.6-4.9 2.6.9-5.5-4-3.9 5.5-.8L12 3.8z"/>',
  trend: '<path d="M4.5 17.5l4.8-4.8 3.6 3 6.6-7.7v9.5h-15z" fill="currentColor" fill-opacity="0.16"/><path d="M4.5 16.5l4.8-4.8 3.6 3 6.6-7.7"/><path d="M15.5 7h4v4"/>',
  refund: '<circle cx="12" cy="12" r="8" fill="currentColor" fill-opacity="0.14"/><path d="M7.5 10.5h7a3.5 3.5 0 1 1 0 7H10"/><path d="M10.5 7.5l-3 3 3 3"/>',
  tag: '<path d="M4.5 11.5V5.8A1.3 1.3 0 0 1 5.8 4.5h5.7l8 8-7 7-8-8z" fill="currentColor" fill-opacity="0.16"/><path d="M4.5 11.5V5.8A1.3 1.3 0 0 1 5.8 4.5h5.7l8 8-7 7-8-8z"/><circle cx="8.5" cy="8.5" r="1.1" fill="currentColor"/>',
};

export const PICKABLE_ICONS: string[] = [
  'bowl', 'coffee', 'breakfast', 'bento', 'fruit', 'dining',
  'bag', 'cart', 'shirt', 'laptop', 'sparkles', 'train',
  'car', 'fuel', 'plane', 'home', 'bolt', 'wifi',
  'film', 'gamepad', 'dumbbell', 'compass', 'medkit', 'book',
  'paw', 'gift', 'wallet', 'star', 'trend', 'tag',
];

export function getIconSvg(iconId?: string): string {
  if (!iconId) return ICON_LIBRARY.tag;
  return ICON_LIBRARY[iconId] || ICON_LIBRARY.tag;
}

export interface CategoryColorPreset {
  color: string;
  bgColor: string;
  label: string;
}

export const CATEGORY_COLOR_PRESETS: CategoryColorPreset[] = [
  { color: '#D6553E', bgColor: '#FDF1EE', label: '赤陶红' },
  { color: '#DE8A1E', bgColor: '#FFF6E8', label: '琥珀金' },
  { color: '#2F78A8', bgColor: '#EDF5FA', label: '海盐蓝' },
  { color: '#3E8E63', bgColor: '#ECF6F0', label: '鼠尾草绿' },
  { color: '#7E57A6', bgColor: '#F4EFF9', label: '鸢尾紫' },
  { color: '#C84B59', bgColor: '#FCEEEF', label: '蔷薇粉' },
  { color: '#456799', bgColor: '#EEF2F8', label: '靛青蓝' },
  { color: '#B0743C', bgColor: '#F9F2EA', label: '焦糖棕' },
  { color: '#C45B7A', bgColor: '#FCF0F4', label: '梅子粉' },
  { color: '#64748B', bgColor: '#F1F5F9', label: '岩板灰' },
];

export const LEDGER_COLOR_PRESETS: string[] = [
  '#DE8A1E',
  '#3E8E63',
  '#3A7CA5',
  '#8A5A9E',
  '#D6553E',
  '#18181B',
];
