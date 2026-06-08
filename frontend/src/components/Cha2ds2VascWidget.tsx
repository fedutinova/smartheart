import { useState } from 'react';

type Sex = 'male' | 'female';

interface RiskFactor {
  key: string;
  /** Подпись чекбокса */
  label: string;
  /** Баллов за фактор */
  points: number;
}

/** Факторы-чекбоксы (возраст и пол считаются отдельно) */
const FACTORS: RiskFactor[] = [
  { key: 'chf', label: 'Хроническая сердечная недостаточность / дисфункция левого желудочка', points: 1 },
  { key: 'hypertension', label: 'Артериальная гипертензия', points: 1 },
  { key: 'diabetes', label: 'Сахарный диабет', points: 1 },
  {
    key: 'stroke',
    label: 'Ишемический инсульт / ТИА / системные эмболии в анамнезе',
    points: 2,
  },
  {
    key: 'vascular',
    label: 'Сосудистое заболевание (инфаркт миокарда, атеросклероз периферических артерий, бляшка в аорте)',
    points: 1,
  },
];

type Risk = {
  label: string;
  recommendation: string;
  className: string;
};

/**
 * Интерпретация суммы баллов CHA2DS2-VASc с учётом пола.
 * Мужчины: 0 — низкий, 1 — умеренный, ≥2 — высокий.
 * Женщины: 1 — низкий, 2 — умеренный, ≥3 — высокий.
 */
function interpret(score: number, sex: Sex): Risk {
  const low = sex === 'male' ? 0 : 1;
  const moderate = sex === 'male' ? 1 : 2;
  if (score <= low) {
    return {
      label: 'Низкий риск',
      recommendation: 'Антикоагулянтная терапия, как правило, не требуется.',
      className: 'bg-green-100 text-green-800',
    };
  }
  if (score === moderate) {
    return {
      label: 'Умеренный риск',
      recommendation: 'Решение о терапии принимается индивидуально.',
      className: 'bg-yellow-100 text-yellow-800',
    };
  }
  return {
    label: 'Высокий риск',
    recommendation: 'Показана антикоагулянтная терапия.',
    className: 'bg-red-100 text-red-800',
  };
}

/** Баллы за возраст: ≥75 → 2, 65–74 → 1, иначе 0 */
function agePoints(age: number): number {
  if (age >= 75) return 2;
  if (age >= 65) return 1;
  return 0;
}

/**
 * Самодостаточный виджет шкалы CHA2DS2-VASc: оценка риска ишемического инсульта
 * при неклапанной фибрилляции предсердий. Не зависит от авторизации и Layout.
 */
export function Cha2ds2VascWidget() {
  const [age, setAge] = useState('');
  const [sex, setSex] = useState<Sex>('male');
  const [checked, setChecked] = useState<Record<string, boolean>>({});

  const ageNum = parseFloat(age);
  const ageValid = Number.isFinite(ageNum) && ageNum >= 0;

  const toggle = (key: string) =>
    setChecked((prev) => ({ ...prev, [key]: !prev[key] }));

  const factorPoints = FACTORS.reduce(
    (sum, f) => sum + (checked[f.key] ? f.points : 0),
    0,
  );
  const ageScore = ageValid ? agePoints(ageNum) : 0;
  const sexScore = sex === 'female' ? 1 : 0;
  const score = factorPoints + ageScore + sexScore;

  const risk = ageValid ? interpret(score, sex) : null;

  return (
    <div>
      {/* Ввод данных */}
      <div className="bg-white rounded-lg shadow border border-gray-100 p-4 sm:p-5 mb-4 space-y-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label htmlFor="age" className="block text-sm font-medium text-gray-700 mb-1">
              Возраст, лет
            </label>
            <input
              id="age"
              type="number"
              inputMode="numeric"
              min={0}
              value={age}
              onChange={(e) => setAge(e.target.value)}
              placeholder="напр. 68"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-transparent"
            />
            <p className="mt-1 text-xs text-gray-400">65–74 года — 1 балл, ≥75 лет — 2 балла</p>
          </div>
          <div>
            <span className="block text-sm font-medium text-gray-700 mb-1">Пол</span>
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
                    sex === opt.value ? 'bg-rose-600 text-white' : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
            <p className="mt-1 text-xs text-gray-400">Женский пол — 1 балл</p>
          </div>
        </div>

        <div className="space-y-2 pt-1">
          {FACTORS.map((f) => (
            <label
              key={f.key}
              className="flex items-start gap-3 rounded-lg border border-gray-200 px-3 py-2.5 cursor-pointer hover:bg-gray-50 transition-colors"
            >
              <input
                type="checkbox"
                checked={!!checked[f.key]}
                onChange={() => toggle(f.key)}
                className="mt-0.5 h-4 w-4 rounded border-gray-300 text-rose-600 focus:ring-rose-500"
              />
              <span className="text-sm text-gray-800 flex-1">{f.label}</span>
              <span className="text-xs text-gray-400 shrink-0 mt-0.5">+{f.points}</span>
            </label>
          ))}
        </div>
      </div>

      {/* Результат */}
      {risk ? (
        <div className="bg-white rounded-lg shadow border border-gray-100 overflow-hidden">
          <div className="px-4 sm:px-5 py-4 border-b border-gray-100 flex items-center justify-between gap-3">
            <div>
              <p className="text-xs text-gray-500">Сумма баллов CHA₂DS₂-VASc</p>
              <p className="text-2xl font-bold text-gray-900">
                {score} <span className="text-base font-normal text-gray-400">/ 9</span>
              </p>
            </div>
            <span className={`px-3 py-1 text-sm font-medium rounded-full ${risk.className}`}>
              {risk.label}
            </span>
          </div>
          <div className="px-4 sm:px-5 py-3 text-sm text-gray-700 leading-relaxed">
            {risk.recommendation}
          </div>
          <div className="px-4 sm:px-5 py-3 border-t border-gray-100 text-[11px] text-gray-400 leading-relaxed">
            Шкала применяется при неклапанной фибрилляции предсердий (ESC, AHA/ACC/HRS).
            Для оценки риска кровотечений дополнительно используйте шкалу HAS-BLED.
            Результат носит справочный характер и не заменяет консультацию врача.
          </div>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow border border-gray-100 px-4 sm:px-5 py-8 text-center text-sm text-gray-400">
          Укажите возраст, чтобы рассчитать риск
        </div>
      )}
    </div>
  );
}
