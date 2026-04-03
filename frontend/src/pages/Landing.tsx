import { Link, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';

// Static demo: ECG analysis result
function DemoPreview() {
  const leads = [
    { name: 'I', r: '4.2', s: '1.0' },
    { name: 'II', r: '8.5', s: '0.5' },
    { name: 'V1', r: '2.0', s: '12.3' },
    { name: 'V5', r: '18.1', s: '1.5' },
  ];

  return (
    <div className="bg-white rounded-2xl shadow-xl border border-gray-200 overflow-hidden">
      <div className="bg-gray-50 border-b border-gray-200 px-4 sm:px-6 py-3 flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">Результат анализа ЭКГ</span>
        <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-green-100 text-green-700">Завершён</span>
      </div>
      <div className="p-4 sm:p-6 space-y-4">
        <div className="flex flex-wrap gap-2">
          <span className="px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-600">Мужской</span>
          <span className="px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-600">52 года</span>
          <span className="px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-600">25 мм/с</span>
        </div>
        <div>
          <p className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-2">Измерения</p>
          <div className="grid grid-cols-4 gap-px bg-gray-200 rounded-lg overflow-hidden text-sm">
            <div className="bg-gray-50 px-3 py-1.5 font-medium text-gray-500">Отв.</div>
            <div className="bg-gray-50 px-3 py-1.5 text-right font-medium text-gray-500">R, мм</div>
            <div className="bg-gray-50 px-3 py-1.5 font-medium text-gray-500">Отв.</div>
            <div className="bg-gray-50 px-3 py-1.5 text-right font-medium text-gray-500">R, мм</div>
            {leads.map((l) => (
              <div key={l.name} className="contents">
                <div className="bg-white px-3 py-1.5 font-medium text-gray-800">{l.name}</div>
                <div className="bg-white px-3 py-1.5 text-right font-mono text-gray-700">{l.r}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="grid grid-cols-3 gap-3">
          <div className="bg-gray-50 rounded-lg p-2.5 text-center">
            <p className="text-[10px] text-gray-400 mb-0.5">QRS</p>
            <p className="text-base font-semibold text-gray-900">94 <span className="text-xs font-normal text-gray-400">мс</span></p>
          </div>
          <div className="bg-gray-50 rounded-lg p-2.5 text-center">
            <p className="text-[10px] text-gray-400 mb-0.5">ЧСС</p>
            <p className="text-base font-semibold text-gray-900">72 <span className="text-xs font-normal text-gray-400">уд/мин</span></p>
          </div>
          <div className="bg-gray-50 rounded-lg p-2.5 text-center">
            <p className="text-[10px] text-gray-400 mb-0.5">Ось</p>
            <p className="text-base font-semibold text-gray-900">+48<span className="text-xs font-normal text-gray-400">°</span></p>
          </div>
        </div>
        <div>
          <p className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-2">Индексы ГЛЖ</p>
          <div className="space-y-1.5">
            {[
              { name: 'Соколов-Лайон', val: '2.81 мВ', ok: true, thr: '< 3.5' },
              { name: 'Корнелл', val: '1.92 мВ', ok: true, thr: '< 2.8' },
              { name: 'Пегуэро', val: '2.45 мВ', ok: false, thr: '≥ 2.3' },
            ].map((idx) => (
              <div key={idx.name} className="flex items-center justify-between text-sm">
                <span className="text-gray-600">{idx.name}</span>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-gray-900">{idx.val}</span>
                  <span className={`text-xs px-1.5 py-0.5 rounded ${idx.ok ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                    {idx.thr}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// Static demo: chat bot dialog
function ChatDemo() {
  return (
    <div className="bg-white rounded-2xl shadow-xl border border-gray-200 overflow-hidden">
      <div className="bg-gray-50 border-b border-gray-200 px-4 sm:px-6 py-3">
        <span className="text-sm font-medium text-gray-700">Чат-бот по кардиологии</span>
      </div>
      <div className="p-4 sm:p-6 space-y-3">
        <div className="flex justify-end">
          <div className="bg-rose-600 text-white rounded-lg px-4 py-2 text-sm max-w-[85%]">
            Как отличить АВ-блокаду II степени Мобитц I от Мобитц II?
          </div>
        </div>
        <div className="flex justify-start">
          <div className="bg-gray-50 border border-gray-200 rounded-lg px-4 py-2 text-sm text-gray-700 max-w-[85%] space-y-2">
            <p>
              <strong>Мобитц I</strong> (периодика Венкебаха): постепенное удлинение PQ
              от комплекса к комплексу, затем выпадение QRS. Интервалы R-R укорачиваются
              перед паузой. Обычно на уровне АВ-узла, чаще доброкачественная.
            </p>
            <p>
              <strong>Мобитц II</strong>: PQ стабильный, QRS выпадает внезапно
              без предшествующего удлинения. Блок ниже АВ-узла (пучок Гиса).
              QRS чаще расширен. Риск прогрессии в полную АВ-блокаду.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

const FEATURES = [
  {
    title: 'Автоматизированный анализ ЭКГ',
    description: 'Загрузите фото ЭКГ и получите структурированные измерения, индексы ГЛЖ и предварительную интерпретацию',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M21 8.25c0-2.485-2.099-4.5-4.688-4.5-1.935 0-3.597 1.126-4.312 2.733-.715-1.607-2.377-2.733-4.313-2.733C5.1 3.75 3 5.765 3 8.25c0 7.22 9 12 9 12s9-4.78 9-12Z" />
      </svg>
    ),
  },
  {
    title: 'Справочный чат-бот',
    description: 'Задайте вопрос по кардиологии. Ответ формируется на основе медицинских учебников и клинических рекомендаций',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
      </svg>
    ),
  },
  {
    title: 'Структурированные измерения',
    description: 'Амплитуды R и S по 12 отведениям, интервалы, ЧСС, ось QRS: всё в удобной таблице',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M3.375 19.5h17.25m-17.25 0a1.125 1.125 0 0 1-1.125-1.125M3.375 19.5h7.5c.621 0 1.125-.504 1.125-1.125m-9.75 0V5.625m0 12.75v-1.5c0-.621.504-1.125 1.125-1.125m18.375 2.625V5.625m0 12.75c0 .621-.504 1.125-1.125 1.125m1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125m0 3.75h-7.5A1.125 1.125 0 0 1 12 18.375m9.75-12.75c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125m19.5 0v1.5c0 .621-.504 1.125-1.125 1.125M2.25 5.625v1.5c0 .621.504 1.125 1.125 1.125m0 0h17.25m-17.25 0h7.5c.621 0 1.125.504 1.125 1.125M3.375 8.25c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125m17.25-3.75h-7.5c-.621 0-1.125.504-1.125 1.125m8.625-1.125c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125M12 10.875v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 10.875c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125M13.125 12h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125M20.625 12c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5M12 14.625v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 14.625c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125m0 0v.375" />
      </svg>
    ),
  },
  {
    title: 'Индексы ГЛЖ',
    description: 'Автоматический расчёт Sokolov-Lyon, Cornell, Peguero, Gubner, Lewis с цветовой индикацией нормы',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z" />
      </svg>
    ),
  },
  {
    title: 'С телефона или компьютера',
    description: 'Сфотографируйте ЭКГ камерой телефона или загрузите скан с компьютера. Обрежьте и поверните при необходимости',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 1.5H8.25A2.25 2.25 0 0 0 6 3.75v16.5a2.25 2.25 0 0 0 2.25 2.25h7.5A2.25 2.25 0 0 0 18 20.25V3.75a2.25 2.25 0 0 0-2.25-2.25H13.5m-3 0V3h3V1.5m-3 0h3m-3 18.75h3" />
      </svg>
    ),
  },
  {
    title: '2 бесплатных анализа в день',
    description: 'Попробуйте без оплаты. Нужно больше? Месячная подписка с безлимитными анализами',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M21 11.25v8.25a1.5 1.5 0 0 1-1.5 1.5H5.25a1.5 1.5 0 0 1-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 1 0 9.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1 1 14.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
      </svg>
    ),
  },
];

const STEPS = [
  { num: '1', title: 'Загрузите фото', description: 'Сфотографируйте ЭКГ или загрузите файл' },
  { num: '2', title: 'Укажите параметры', description: 'Возраст, пол, скорость плёнки' },
  { num: '3', title: 'Получите результат', description: 'Измерения, индексы и справочную интерпретацию' },
];

export function Landing() {
  const { isAuthenticated } = useAuthStore();

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  return (
    <div className="min-h-screen bg-white">
      {/* Header */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 sm:h-16 flex items-center justify-between">
          <span className="text-lg sm:text-xl text-rose-600 shrink-0" style={{ fontFamily: "'Prosto One', cursive" }}>
            Умное сердце
          </span>
          <div className="flex items-center gap-2 sm:gap-3">
            <Link
              to={ROUTES.LOGIN}
              className="text-sm text-gray-600 hover:text-gray-900 transition-colors px-2 sm:px-3 py-2 whitespace-nowrap"
            >
              Войти
            </Link>
            <Link
              to={ROUTES.REGISTER}
              className="text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 px-3 sm:px-4 py-2 rounded-lg transition-colors whitespace-nowrap"
            >
              Начать
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="pt-32 pb-16 sm:pt-40 sm:pb-24 px-4 sm:px-6">
        <div className="max-w-3xl mx-auto text-center animate-fade-in-up">
          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 tracking-tight leading-tight">
            Анализ ЭКГ с помощью{' '}
            <span className="text-rose-600">искусственного интеллекта</span>
          </h1>
          <p className="mt-6 text-lg sm:text-xl text-gray-500 max-w-2xl mx-auto leading-relaxed">
            Загрузите фото электрокардиограммы и получите структурированные измерения,
            расчёт индексов гипертрофии и справочную интерпретацию
          </p>
          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              to={ROUTES.REGISTER}
              className="w-full sm:w-auto px-8 py-3.5 text-base font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
            >
              Попробовать бесплатно
            </Link>
            <a
              href="#how-it-works"
              className="w-full sm:w-auto px-8 py-3.5 text-base font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 active:scale-95 rounded-xl transition-all duration-150"
            >
              Как это работает
            </a>
          </div>
          <p className="mt-4 text-sm text-gray-400">
            2 бесплатных анализа в день, без привязки карты
          </p>
          <p className="mt-2 text-xs text-gray-400 max-w-2xl mx-auto">
            Сервис предназначен для информационной поддержки и не заменяет медицинское заключение врача.
          </p>
        </div>
      </section>

      {/* Product demos */}
      <section className="pb-16 sm:pb-20 px-4 sm:px-6">
        <div className="max-w-5xl mx-auto grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div>
            <p className="text-sm text-gray-400 mb-3">Пример результата анализа</p>
            <DemoPreview />
          </div>
          <div>
            <p className="text-sm text-gray-400 mb-3">Пример работы чат-бота</p>
            <ChatDemo />
          </div>
        </div>
      </section>

      {/* How it works — numbered timeline */}
      <section id="how-it-works" className="py-16 sm:py-20 bg-gray-50 px-4 sm:px-6">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 text-center mb-12">
            Как это работает
          </h2>
          {/* Desktop: horizontal */}
          <div className="hidden sm:flex items-start">
            {STEPS.map((step, i) => (
              <div key={step.num} className="flex items-start flex-1">
                <div className="flex flex-col items-center flex-1">
                  <div className="w-10 h-10 rounded-full bg-rose-600 text-white text-sm font-bold flex items-center justify-center">
                    {step.num}
                  </div>
                  <h3 className="text-sm font-semibold text-gray-900 mt-3 mb-1">{step.title}</h3>
                  <p className="text-xs text-gray-500 text-center px-2">{step.description}</p>
                </div>
                {i < STEPS.length - 1 && (
                  <div className="flex-shrink-0 w-12 flex items-center justify-center mt-5">
                    <div className="w-full h-px bg-gray-300" />
                  </div>
                )}
              </div>
            ))}
          </div>
          {/* Mobile: vertical */}
          <div className="sm:hidden space-y-0">
            {STEPS.map((step, i) => (
              <div key={step.num} className="flex gap-4">
                <div className="flex flex-col items-center">
                  <div className="w-9 h-9 rounded-full bg-rose-600 text-white text-sm font-bold flex items-center justify-center shrink-0">
                    {step.num}
                  </div>
                  {i < STEPS.length - 1 && <div className="w-px h-8 bg-gray-300" />}
                </div>
                <div className="pb-8">
                  <h3 className="text-sm font-semibold text-gray-900">{step.title}</h3>
                  <p className="text-xs text-gray-500 mt-0.5">{step.description}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Features — compact list */}
      <section className="py-16 sm:py-20 px-4 sm:px-6">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 text-center mb-12">
            Возможности
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-12 gap-y-6">
            {FEATURES.map((feature) => (
              <div key={feature.title} className="flex gap-4 items-start">
                <div className="w-9 h-9 rounded-lg bg-rose-100 text-rose-600 flex items-center justify-center shrink-0 mt-0.5">
                  {feature.icon}
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-gray-900">{feature.title}</h3>
                  <p className="text-sm text-gray-500 mt-0.5 leading-relaxed">{feature.description}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 sm:py-20 px-4 sm:px-6 bg-gray-50">
        <div className="max-w-2xl mx-auto text-center">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 mb-3">
            Начните прямо сейчас
          </h2>
          <p className="text-gray-500 mb-8">
            2 бесплатных анализа каждый день. Регистрация за 30 секунд.
          </p>
          <Link
            to={ROUTES.REGISTER}
            className="inline-block px-8 py-3.5 text-base font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
          >
            Создать аккаунт
          </Link>
          <p className="mt-4 text-sm text-gray-400">
            Уже есть аккаунт?{' '}
            <Link to={ROUTES.LOGIN} className="text-rose-600 hover:text-rose-700">
              Войти
            </Link>
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-100 py-8 px-4 sm:px-6">
        <div className="max-w-6xl mx-auto flex flex-col items-center gap-4 text-xs text-gray-400 sm:flex-row sm:justify-between">
          <span className="text-center sm:text-left">Самозанятая Федутинова А.А., ИНН 575212369164</span>
          <div className="flex flex-wrap justify-center gap-x-4 gap-y-1">
            <Link to={ROUTES.CONTACTS} className="hover:text-gray-600 transition-colors">О нас</Link>
            <Link to={ROUTES.TERMS} className="hover:text-gray-600 transition-colors">Оферта</Link>
            <Link to={ROUTES.PRIVACY} className="hover:text-gray-600 transition-colors">Конфиденциальность</Link>
            <a href="mailto:support@smartheart.online" className="hover:text-gray-600 transition-colors">Поддержка</a>
          </div>
        </div>
        <p className="max-w-6xl mx-auto mt-4 text-center text-[11px] text-gray-300 leading-relaxed">
          Сервис не является медицинским изделием. Результаты носят информационный характер и не заменяют консультацию врача.
        </p>
      </footer>
    </div>
  );
}
