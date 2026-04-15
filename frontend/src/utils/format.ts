import { format, formatDistanceToNow } from 'date-fns';
import { ru } from 'date-fns/locale';
import type { Request } from '@/types';

export const formatDate = (date: string | Date): string => {
  return format(new Date(date), 'dd.MM.yyyy HH:mm');
};

/** "15 апреля 2026" — для профиля, подписки и т.п. */
export const formatDateLong = (date: string | Date): string => {
  return format(new Date(date), 'd MMMM yyyy', { locale: ru });
};

export const formatRelative = (date: string | Date): string => {
  return formatDistanceToNow(new Date(date), { addSuffix: true, locale: ru });
};

const STATUS_LABELS: Record<string, string> = {
  queued: 'В очереди',
  started: 'Выполняется',
  processing: 'Обрабатывается',
  succeeded: 'Завершено',
  completed: 'Завершено',
  failed: 'Ошибка',
  pending: 'Ожидает',
};

const STATUS_COLORS: Record<string, string> = {
  queued: 'bg-blue-100 text-blue-800',
  started: 'bg-yellow-100 text-yellow-800',
  processing: 'bg-yellow-100 text-yellow-800',
  succeeded: 'bg-green-100 text-green-800',
  completed: 'bg-green-100 text-green-800',
  failed: 'bg-red-100 text-red-800',
  pending: 'bg-gray-100 text-gray-800',
};

export const formatStatus = (status: string): string =>
  STATUS_LABELS[status] || status;

export const getStatusColor = (status: string): string =>
  STATUS_COLORS[status] || 'bg-gray-100 text-gray-800';

export const formatPrice = (kopecks: number): string => {
  const rub = Math.floor(kopecks / 100);
  const kop = kopecks % 100;
  return kop === 0 ? `${rub}` : `${rub}.${String(kop).padStart(2, '0')}`;
};

export const formatECGParams = (r: Request): string => {
  const parts: string[] = [];
  if (r.ecg_sex === 'male') parts.push('М');
  else if (r.ecg_sex === 'female') parts.push('Ж');
  if (r.ecg_age) parts.push(`${r.ecg_age} лет`);
  if (r.ecg_paper_speed_mms) parts.push(`${r.ecg_paper_speed_mms} мм/с`);
  if (r.ecg_mm_per_mv_limb && r.ecg_mm_per_mv_chest) {
    if (r.ecg_mm_per_mv_limb === r.ecg_mm_per_mv_chest) {
      parts.push(`${r.ecg_mm_per_mv_limb} мм/мВ`);
    } else {
      parts.push(`${r.ecg_mm_per_mv_limb}/${r.ecg_mm_per_mv_chest} мм/мВ`);
    }
  }
  return parts.join(' · ');
};

