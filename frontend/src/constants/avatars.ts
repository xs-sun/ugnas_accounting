import cat1 from '@/assets/avatars/cat-1.jpg';
import cat2 from '@/assets/avatars/cat-2.jpg';
import cat3 from '@/assets/avatars/cat-3.jpg';
import cat4 from '@/assets/avatars/cat-4.jpg';
import cat5 from '@/assets/avatars/cat-5.jpg';
import cat6 from '@/assets/avatars/cat-6.jpg';
import logoMascot from '@/assets/logo.png';

export interface CatAvatarPreset {
  id: string;
  name: string;
  src: string;
  publicPath: string;
}

export const CAT_AVATARS: CatAvatarPreset[] = [
  { id: 'cat_grey', name: '小银灰', src: cat1, publicPath: '/avatars/cat-1.jpg' },
  { id: 'cat_orange', name: '大橘宝', src: cat2, publicPath: '/avatars/cat-2.jpg' },
  { id: 'cat_calico', name: '小三花', src: cat3, publicPath: '/avatars/cat-3.jpg' },
  { id: 'cat_ragdoll', name: '小布偶', src: cat4, publicPath: '/avatars/cat-4.jpg' },
  { id: 'cat_tuxedo', name: '奶牛猫', src: cat5, publicPath: '/avatars/cat-5.jpg' },
  { id: 'cat_black', name: '招财黑猫', src: cat6, publicPath: '/avatars/cat-6.jpg' },
];

export const DEFAULT_MASCOT: string = logoMascot;

export const MASCOT_FALLBACK =
  "data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120'><circle cx='60' cy='60' r='58' fill='%23FFF7E8'/><circle cx='28' cy='64' r='10' fill='%233E8E63' opacity='0.88'/><circle cx='92' cy='62' r='10' fill='%23D6553E' opacity='0.88'/><rect x='34' y='56' width='52' height='46' rx='10' fill='%23F2AC42' stroke='%23C8761A' stroke-width='2.5'/><rect x='30' y='66' width='8' height='5' rx='2.5' fill='%236B3D14'/><rect x='30' y='76' width='8' height='5' rx='2.5' fill='%236B3D14'/><rect x='30' y='86' width='8' height='5' rx='2.5' fill='%236B3D14'/><polygon points='41,42 46,24 56,36' fill='%23A6A2A0'/><polygon points='79,42 74,24 64,36' fill='%23A6A2A0'/><ellipse cx='60' cy='46' rx='21' ry='16' fill='%23B8B4B2'/><circle cx='52' cy='46' r='2.8' fill='%23231B14'/><circle cx='68' cy='46' r='2.8' fill='%23231B14'/><path d='M57 51 Q60 54 63 51' fill='none' stroke='%235C3416' stroke-width='2' stroke-linecap='round'/><text x='60' y='85' text-anchor='middle' font-size='15' font-weight='bold' fill='%234A2B0E' font-family='sans-serif'>七猫</text></svg>";

export function getRandomCatAvatar(excludeSrc?: string): string {
  const candidates = excludeSrc
    ? CAT_AVATARS.filter((item) => item.src !== excludeSrc && item.publicPath !== excludeSrc)
    : CAT_AVATARS;
  const pool = candidates.length > 0 ? candidates : CAT_AVATARS;
  return pool[Math.floor(Math.random() * pool.length)].src;
}

export function resolveAvatarUrl(avatar?: string | null): string {
  if (!avatar) return CAT_AVATARS[0].src;
  const trimmed = avatar.trim();
  if (!trimmed) return CAT_AVATARS[0].src;

  const byPreset = CAT_AVATARS.find(
    (item) =>
      item.id === trimmed ||
      item.publicPath === trimmed ||
      `./${item.publicPath.replace(/^\//, '')}` === trimmed ||
      trimmed.endsWith(item.publicPath)
  );
  if (byPreset) return byPreset.src;

  return trimmed;
}
