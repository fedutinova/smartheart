interface SkeletonProps {
  className?: string;
}

function Bone({ className = '' }: SkeletonProps) {
  return <div className={`animate-pulse rounded bg-gray-200 ${className}`} />;
}

export function AccountSkeleton() {
  return (
    <div className="max-w-4xl mx-auto">
      <Bone className="h-8 w-48 mb-6" />

      {/* Profile card */}
      <div className="bg-white shadow rounded-xl p-6 mb-6">
        <Bone className="h-4 w-16 mb-4" />
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i}>
              <Bone className="h-3 w-12 mb-1.5" />
              <Bone className="h-4 w-32" />
            </div>
          ))}
        </div>
      </div>

      {/* Subscription card */}
      <div className="bg-white shadow rounded-xl p-6 mb-6">
        <Bone className="h-4 w-20 mb-4" />
        <div className="flex items-center justify-between">
          <div>
            <Bone className="h-4 w-40 mb-1.5" />
            <Bone className="h-3 w-56" />
          </div>
          <Bone className="h-9 w-24 rounded-lg" />
        </div>
      </div>

      {/* Quota card */}
      <div className="bg-white shadow rounded-xl p-6 mb-6">
        <Bone className="h-4 w-20 mb-4" />
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {[1, 2].map((i) => (
            <div key={i} className="bg-gray-50 rounded-lg p-4 text-center">
              <Bone className="h-9 w-12 mx-auto mb-1" />
              <Bone className="h-3 w-28 mx-auto" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function DashboardHistorySkeleton() {
  return (
    <>
      {/* Mobile skeleton */}
      <div className="sm:hidden divide-y divide-gray-200">
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="px-4 py-3 flex items-center justify-between">
            <div className="min-w-0 flex-1">
              <Bone className="h-4 w-32 mb-1.5" />
              <Bone className="h-3 w-20" />
            </div>
            <Bone className="ml-3 h-5 w-20 rounded-full" />
          </div>
        ))}
      </div>

      {/* Desktop skeleton */}
      <div className="hidden sm:block">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {['Параметры', 'Статус', 'Создано', 'Действия'].map((h) => (
                <th key={h} className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {[1, 2, 3, 4, 5].map((i) => (
              <tr key={i}>
                <td className="px-6 py-4"><Bone className="h-4 w-28" /></td>
                <td className="px-6 py-4"><Bone className="h-5 w-20 rounded-full" /></td>
                <td className="px-6 py-4"><Bone className="h-4 w-24" /></td>
                <td className="px-6 py-4"><Bone className="h-4 w-16" /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

export function HistoryTableSkeleton() {
  return (
    <>
      {/* Mobile skeleton */}
      <div className="sm:hidden divide-y divide-gray-200">
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="px-4 py-3">
            <div className="flex items-center justify-between mb-1">
              <Bone className="h-4 w-28" />
              <Bone className="h-5 w-20 rounded-full" />
            </div>
            <Bone className="h-3 w-24" />
          </div>
        ))}
      </div>

      {/* Desktop skeleton */}
      <div className="hidden sm:block overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {['Параметры', 'Статус', 'Создано', 'Обновлено', 'Действия'].map((h) => (
                <th key={h} className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {[1, 2, 3, 4, 5].map((i) => (
              <tr key={i}>
                <td className="px-6 py-4"><Bone className="h-4 w-28" /></td>
                <td className="px-6 py-4"><Bone className="h-5 w-20 rounded-full" /></td>
                <td className="px-6 py-4"><Bone className="h-4 w-24" /></td>
                <td className="px-6 py-4"><Bone className="h-4 w-24" /></td>
                <td className="px-6 py-4 text-right"><Bone className="h-4 w-16 ml-auto" /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
