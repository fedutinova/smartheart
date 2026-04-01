interface CalibrationFormProps {
  age: string;
  sex: string;
  paperSpeed: number;
  mmPerMvLimb: number;
  mmPerMvChest: number;
  onAgeChange: (v: string) => void;
  onSexChange: (v: string) => void;
  onPaperSpeedChange: (v: number) => void;
  onMmPerMvLimbChange: (v: number) => void;
  onMmPerMvChestChange: (v: number) => void;
}

function ToggleGroup({ value, options, onChange }: {
  value: number | string;
  options: { value: number | string; label: string }[];
  onChange: (v: any) => void;
}) {
  return (
    <div className="flex rounded-lg bg-white ring-1 ring-gray-200 p-0.5">
      {options.map((opt) => (
        <button
          key={String(opt.value)}
          type="button"
          onClick={() => onChange(opt.value)}
          className={`flex-1 text-sm py-1.5 rounded-md transition-all ${
            value === opt.value
              ? 'bg-rose-500 text-white shadow-sm font-medium'
              : 'text-gray-800 hover:text-gray-900'
          }`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

export function CalibrationForm({
  age, sex, paperSpeed, mmPerMvLimb, mmPerMvChest,
  onAgeChange, onSexChange, onPaperSpeedChange, onMmPerMvLimbChange, onMmPerMvChestChange,
}: CalibrationFormProps) {
  return (
    <div className="rounded-xl bg-gray-50 p-4 space-y-4">
      <div className="flex items-center gap-4">
        <div className="flex-1 min-w-0">
          <label htmlFor="age" className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Возраст</label>
          <input
            id="age"
            type="number"
            min={1}
            max={150}
            placeholder="—"
            className="w-full bg-white rounded-lg border-0 ring-1 ring-gray-200 focus:ring-2 focus:ring-rose-500 text-sm py-2 px-3"
            value={age}
            onChange={(e) => onAgeChange(e.target.value)}
          />
        </div>
        <div className="flex-1 min-w-0">
          <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Пол</label>
          <ToggleGroup
            value={sex}
            options={[
              { value: '', label: '—' },
              { value: 'male', label: 'М' },
              { value: 'female', label: 'Ж' },
            ]}
            onChange={onSexChange}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">Скорость мм/с</label>
          <ToggleGroup value={paperSpeed} options={[{ value: 25, label: '25' }, { value: 50, label: '50' }]} onChange={onPaperSpeedChange} />
        </div>
        <div>
          <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">мм/мВ конечн.</label>
          <ToggleGroup value={mmPerMvLimb} options={[5, 10, 20].map((v) => ({ value: v, label: String(v) }))} onChange={onMmPerMvLimbChange} />
        </div>
        <div>
          <label className="block text-[11px] uppercase tracking-wide text-gray-600 font-medium mb-1.5">мм/мВ грудные</label>
          <ToggleGroup value={mmPerMvChest} options={[5, 10, 20].map((v) => ({ value: v, label: String(v) }))} onChange={onMmPerMvChestChange} />
        </div>
      </div>
    </div>
  );
}
