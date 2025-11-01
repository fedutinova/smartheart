import { format, formatDistanceToNow } from 'date-fns';

export const formatDate = (date: string | Date): string => {
  return format(new Date(date), 'dd.MM.yyyy HH:mm');
};

export const formatRelative = (date: string | Date): string => {
  return formatDistanceToNow(new Date(date), { addSuffix: true });
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

