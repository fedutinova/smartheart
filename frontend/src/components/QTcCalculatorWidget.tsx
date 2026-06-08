import { useState, useMemo } from 'react';

type Sex = 'male' | 'female';

interface FormulaResult {
  key: string;
  name: string;
  description: string;
  /** QTc в миллисекундах */
  qtc: number;
}

type Interpretation = {
  label: string;
  /** Tailwind-классы для бейджа */
  className: string;
};

/**
 * Диагностические границы QTc (мс) по полу.
 * Мужчины: норма 340–450, укорочение < 340, удлинение > 450.
 * Женщины: норма 340–460, укорочение < 340, удлинение > 460.
 */
const SHORT_QTC = 340;
const PROLONGED_QTC: Record<Sex, number> = {
  male: 450,
  female: 460,
};

function interpret(qtc: number, sex: Sex): Interpretation {
  if (qtc < SHORT_QTC) {
    return { label: 'Укорочение', className: 'bg-amber-100 text-amber-800' };
  }
  if (qtc > PROLONGED_QTC[sex]) {
    return { label: 'Удлинение', className: 'bg-red-100 text-red-800' };
  }
  return { label: 'Норма', className: 'bg-green-100 text-green-800' };
}

/**
 * Расчёт QTc по четырём формулам.
 * @param qtMs интервал QT в мс
 * @param rrSec интервал RR в секундах
 * @param hr ЧСС в уд/мин
 */
function computeFormulas(qtMs: number, rrSec: number, hr: number): FormulaResult[] {
  return [
    {
      key: 'bazett',
      name: 'Базетта',
      description: 'QT / √RR',
      qtc: qtMs / Math.sqrt(rrSec),
    },
    {
      key: 'fridericia',
      name: 'Фридерисии',
      description: 'QT / ∛RR',
      qtc: qtMs / Math.cbrt(rrSec),
    },
    {
      key: 'framingham',
      name: 'Фрамингема',
      description: 'QT + 154 × (1 − RR)',
      qtc: qtMs + 154 * (1 - rrSec),
    },
    {
      key: 'hodges',
      name: 'Ходжеса',
      description: 'QT + 1.75 × (ЧСС − 60)',
      qtc: qtMs + 1.75 * (hr - 60),
    },
  ];
}

/**
 * Самодостаточный виджет калькулятора QTc: поля ввода + результаты.
 * Не зависит от авторизации и Layout — используется и в приложении, и на лендинге.
 */
export function QTcCalculatorWidget() {
  // Поля хранятся как строки, чтобы поле можно было очистить
  const [qt, setQt] = useState('');
  const [hr, setHr] = useState('');
  const [sex, setSex] = useState<Sex>('male');

  const qtNum = parseFloat(qt);
  const hrNum = parseFloat(hr);
  const qtValid = Number.isFinite(qtNum) && qtNum > 0;
  const hrValid = Number.isFinite(hrNum) && hrNum > 0;

  // RR (сек) вычисляется автоматически из ЧСС
  const rrSec = hrValid ? 60 / hrNum : NaN;

  const results = useMemo(() => {
    if (!qtValid || !hrValid) return null;
    return computeFormulas(qtNum, rrSec, hrNum);
  }, [qtValid, hrValid, qtNum, rrSec, hrNum]);

  return (
    <div>
      {/* Inputs */}
      <div className="bg-white rounded-lg shadow border border-gray-100 p-4 sm:p-5 mb-4">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div>
            <label htmlFor="qt" className="block text-sm font-medium text-gray-700 mb-1">
              QT, мс
            </label>
            <input
              id="qt"
              type="number"
              inputMode="decimal"
              min={0}
              value={qt}
              onChange={(e) => setQt(e.target.value)}
              placeholder="напр. 400"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-transparent"
            />
          </div>
          <div>
            <label htmlFor="hr" className="block text-sm font-medium text-gray-700 mb-1">
              ЧСС, уд/мин
            </label>
            <input
              id="hr"
              type="number"
              inputMode="decimal"
              min={0}
              value={hr}
              onChange={(e) => setHr(e.target.value)}
              placeholder="напр. 70"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-transparent"
            />
          </div>
          <div>
            <label htmlFor="rr" className="block text-sm font-medium text-gray-700 mb-1">
              RR, сек <span className="text-gray-400 font-normal">(авто)</span>
            </label>
            <input
              id="rr"
              type="text"
              readOnly
              value={hrValid ? rrSec.toFixed(3) : ''}
              placeholder="60 / ЧСС"
              className="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600"
            />
          </div>
        </div>

        {/* Sex selector — влияет на интерпретацию */}
        <div className="mt-4">
          <span className="block text-sm font-medium text-gray-700 mb-1.5">Пол (для интерпретации)</span>
          <div className="inline-flex rounded-lg border border-gray-300 p-0.5">
            {([
              { value: 'male', label: 'Мужской' },
              { value: 'female', label: 'Женский' },
            ] as const).map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setSex(opt.value)}
                className={`px-4 py-1.5 text-sm rounded-md transition-colors ${
                  sex === opt.value
                    ? 'bg-rose-600 text-white'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Results */}
      {results ? (
        <div className="bg-white rounded-lg shadow border border-gray-100 overflow-hidden">
          <div className="px-4 sm:px-5 py-3 border-b border-gray-100">
            <h2 className="text-sm font-semibold text-gray-900">Результат</h2>
          </div>
          <ul className="divide-y divide-gray-100">
            {results.map((r) => {
              const itp = interpret(r.qtc, sex);
              return (
                <li key={r.key} className="flex items-center justify-between gap-3 px-4 sm:px-5 py-3">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-gray-900">{r.name}</p>
                    <p className="text-xs text-gray-400 truncate">{r.description}</p>
                  </div>
                  <div className="flex items-center gap-3 shrink-0">
                    <span className="text-base font-semibold text-gray-900 tabular-nums">
                      {Math.round(r.qtc)} <span className="text-xs font-normal text-gray-400">мс</span>
                    </span>
                    <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${itp.className}`}>
                      {itp.label}
                    </span>
                  </div>
                </li>
              );
            })}
          </ul>
          <div className="px-4 sm:px-5 py-3 border-t border-gray-100 text-[11px] text-gray-400 leading-relaxed">
            Границы QTc (мс), {sex === 'male' ? 'мужчины' : 'женщины'}: норма {SHORT_QTC}–{PROLONGED_QTC[sex]},
            укорочение &lt;{SHORT_QTC}, удлинение &gt;{PROLONGED_QTC[sex]}.
            Чаще всего применяется формула Базетта, но при высокой или низкой ЧСС она может
            переоценивать или недооценивать интервал. Результаты носят справочный характер и не заменяют консультацию врача.
          </div>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow border border-gray-100 px-4 sm:px-5 py-8 text-center text-sm text-gray-400">
          Введите QT и ЧСС, чтобы рассчитать QTc
        </div>
      )}
    </div>
  );
}
