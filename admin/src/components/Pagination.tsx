interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number) => void;
}

export function Pagination({ total, limit, offset, onChange }: PaginationProps) {
  const totalPages = Math.ceil(total / limit);
  const currentPage = Math.floor(offset / limit) + 1;

  if (totalPages <= 1) return null;

  return (
    <div className="flex items-center justify-between mt-4 text-sm text-gray-600">
      <span>Всего: {total}</span>
      <div className="flex items-center gap-2">
        <button
          onClick={() => onChange(Math.max(0, offset - limit))}
          disabled={offset === 0}
          className="px-3 py-1 rounded border border-gray-300 disabled:opacity-40 hover:bg-gray-100"
        >
          Назад
        </button>
        <span>{currentPage} / {totalPages}</span>
        <button
          onClick={() => onChange(offset + limit)}
          disabled={offset + limit >= total}
          className="px-3 py-1 rounded border border-gray-300 disabled:opacity-40 hover:bg-gray-100"
        >
          Далее
        </button>
      </div>
    </div>
  );
}
