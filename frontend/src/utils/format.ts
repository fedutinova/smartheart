import { format, formatDistanceToNow } from 'date-fns';
import { ru } from 'date-fns/locale';
import type { Request } from '@/types';

export const formatDate = (date: string | Date): string => {
  return format(new Date(date), 'dd.MM.yyyy HH:mm');
};

export const formatRelative = (date: string | Date): string => {
  return formatDistanceToNow(new Date(date), { addSuffix: true, locale: ru });
};

export const formatStatus = (status: string): string => {
  const statusMap: Record<string, string> = {
    queued: 'В очереди',
    started: 'Выполняется',
    processing: 'Обрабатывается',
    succeeded: 'Завершено',
    completed: 'Завершено',
    failed: 'Ошибка',
    pending: 'Ожидает',
  };
  return statusMap[status] || status;
};

export const getStatusColor = (status: string): string => {
  const colorMap: Record<string, string> = {
    queued: 'bg-blue-100 text-blue-800',
    started: 'bg-yellow-100 text-yellow-800',
    processing: 'bg-yellow-100 text-yellow-800',
    succeeded: 'bg-green-100 text-green-800',
    completed: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
    pending: 'bg-gray-100 text-gray-800',
  };
  return colorMap[status] || 'bg-gray-100 text-gray-800';
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

