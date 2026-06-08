import { useState } from 'react';

interface KillipClass {
  /** Римский номер класса */
  roman: string;
  /** Короткая клиническая картина — текст варианта выбора */
  option: string;
  /** Полное описание класса */
  description: string;
  /** Госпитальная летальность */
  mortality: string;
  /** Tailwind-классы для бейджа тяжести */
  badge: string;
}

/**
 * Классификация Killip — стратификация риска при остром инфаркте миокарда
 * по признакам острой сердечной недостаточности.
 * Летальность по современным регистрам (Mello BH et al., Arq Bras Cardiol, 2014).
 */
const KILLIP_CLASSES: KillipClass[] = [
  {
    roman: 'I',
    option: 'Нет признаков сердечной недостаточности',
    description: 'Отсутствие признаков сердечной недостаточности: АД стабильное, лёгкие чистые, хрипов нет, симптомов застоя нет.',
    mortality: '2–3%',
    badge: 'bg-green-100 text-green-800',
  },
  {
    roman: 'II',
    option: 'III тон, влажные хрипы в нижних отделах лёгких',
    description: 'Появление S3 (III тона), умеренные влажные хрипы в нижних отделах лёгких, признаки венозного застоя.',
    mortality: '5–12%',
    badge: 'bg-yellow-100 text-yellow-800',
  },
  {
    roman: 'III',
    option: 'Отёк лёгких, влажные хрипы выше углов лопаток',
    description: 'Отёк лёгких: обильные хрипы более чем над половиной лёгочных полей, тяжёлое дыхание в покое.',
    mortality: '10–20%',
    badge: 'bg-orange-100 text-orange-800',
  },
  {
    roman: 'IV',
    option: 'Кардиогенный шок',
    description: 'Кардиогенный шок: гипотензия (САД < 90 мм рт. ст.), признаки тканевой гипоперфузии (похолодание конечностей, спутанность сознания, олигурия).',
    mortality: '50–81%',
    badge: 'bg-red-100 text-red-800',
  },
];

/**
 * Самодостаточный виджет классификации Killip: выбор клинической картины + результат.
 * Не зависит от авторизации и Layout.
 */
export function KillipWidget() {
  const [selected, setSelected] = useState<number | null>(null);
  const result = selected !== null ? KILLIP_CLASSES[selected] : null;

  return (
    <div>
      {/* Выбор клинической картины */}
      <div className="bg-white rounded-lg shadow border border-gray-100 p-4 sm:p-5 mb-4">
        <p className="block text-sm font-medium text-gray-700 mb-3">
          Выберите наиболее точное описание клинической картины пациента при инфаркте миокарда:
        </p>
        <div className="space-y-2">
          {KILLIP_CLASSES.map((c, i) => {
            const active = selected === i;
            return (
              <button
                key={c.roman}
                type="button"
                onClick={() => setSelected(i)}
                className={`w-full flex items-center gap-3 text-left rounded-lg border px-3 py-2.5 transition-colors ${
                  active
                    ? 'border-rose-500 bg-rose-50'
                    : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                }`}
              >
                <span
                  className={`w-5 h-5 rounded-full border flex items-center justify-center shrink-0 ${
                    active ? 'border-rose-500' : 'border-gray-300'
                  }`}
                >
                  {active && <span className="w-2.5 h-2.5 rounded-full bg-rose-500" />}
                </span>
                <span className="text-sm text-gray-800">{c.option}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Результат */}
      {result ? (
        <div className="bg-white rounded-lg shadow border border-gray-100 overflow-hidden">
          <div className="px-4 sm:px-5 py-4 border-b border-gray-100 flex items-center justify-between gap-3">
            <div>
              <p className="text-xs text-gray-500">Класс по Killip</p>
              <p className="text-2xl font-bold text-gray-900">Класс {result.roman}</p>
            </div>
            <span className={`px-3 py-1 text-sm font-medium rounded-full ${result.badge}`}>
              Летальность {result.mortality}
            </span>
          </div>
          <div className="px-4 sm:px-5 py-3 text-sm text-gray-700 leading-relaxed">
            {result.description}
          </div>
          <div className="px-4 sm:px-5 py-3 border-t border-gray-100 text-[11px] text-gray-400 leading-relaxed">
            Госпитальная летальность по современным регистрам (Mello BH et al., Arq Bras Cardiol, 2014).
            Шкала применяется при поступлении пациента с ОИМ, для решения о реперфузии и интенсивной терапии,
            а также входит в прогностические модели (GRACE). Результат носит справочный характер и не заменяет
            консультацию врача.
          </div>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow border border-gray-100 px-4 sm:px-5 py-8 text-center text-sm text-gray-400">
          Выберите вариант, чтобы определить класс по Killip
        </div>
      )}
    </div>
  );
}
