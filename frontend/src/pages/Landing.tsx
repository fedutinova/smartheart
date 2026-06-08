import { Link, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';
import { SurveyBanner } from '@/components/SurveyBanner';

// Static demo: ECG analysis result (mirrors StructuredResultView interpretation block)
function DemoPreview() {
  return (
    <div className="bg-gradient-to-br from-purple-50 to-blue-50 border border-purple-200 rounded-2xl shadow-xl p-4 sm:p-6">
      <h2 className="text-base font-bold text-gray-900 mb-3">Интерпретация</h2>
      <div className="bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 mb-3 text-xs text-amber-800">
        Результат автоматической обработки. Не является медицинским заключением.
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 mb-3">
        {[
          { label: 'Ритм', value: 'Синусовый', status: 'normal' },
          { label: 'ЭОС', value: 'Нормальная', status: 'normal' },
          { label: 'ГЛЖ', value: 'Пограничная', status: 'abnormal' },
        ].map((s) => (
          <div key={s.label} className="bg-white rounded-lg px-3 py-2.5 border border-purple-100 flex items-center justify-between gap-2">
            <div>
              <p className="text-[10px] text-gray-500">{s.label}</p>
              <p className="text-xs font-medium text-gray-900">{s.value}</p>
            </div>
            <span className={`text-[10px] px-1.5 py-0.5 rounded whitespace-nowrap ${s.status === 'normal' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
              {s.status === 'normal' ? 'норма' : 'отклонение'}
            </span>
          </div>
        ))}
      </div>
      <p className="text-[10px] font-medium text-gray-500 mb-1.5">Критерии ГЛЖ</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {[
          { label: 'Соколов-Лайон', value: '2.81 мВ', status: 'negative', threshold: '< 3.5 мВ' },
          { label: 'Корнелл', value: '1.92 мВ', status: 'negative', threshold: '< 2.8 мВ' },
          { label: 'Пегуэро', value: '2.45 мВ', status: 'positive', threshold: '\u2265 2.3 мВ' },
        ].map((it) => (
          <div key={it.label} className="bg-white rounded-lg px-3 py-2.5 border border-purple-100">
            <div className="flex items-center justify-between">
              <p className="text-xs font-medium text-gray-900">{it.label}: {it.value}</p>
              <span className={`text-[10px] px-1.5 py-0.5 rounded whitespace-nowrap ${it.status === 'negative' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                {it.status === 'negative' ? 'отрицательный' : 'положительный'}
              </span>
            </div>
            <p className="text-[10px] text-gray-400 mt-0.5">{it.threshold}</p>
          </div>
        ))}
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
    description: 'Автоматический расчёт индексов Соколова-Лайона, Корнельского, Пегеро-Ло Прести, Губнера, Льюиса с цветовой индикацией нормы',
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
    title: 'Конфиденциальность данных',
    description: 'Перед анализом изображение автоматически обезличивается: ФИО и персональные данные пациента удаляются',
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" />
      </svg>
    ),
  },
];

const STEPS = [
  { num: '1', title: 'Загрузите фото', description: 'Сфотографируйте ЭКГ или загрузите файл' },
  { num: '2', title: 'Укажите параметры', description: 'Возраст, пол, скорость плёнки' },
  { num: '3', title: 'Получите результат', description: 'Измерения, индексы и справочную интерпретацию' },
  { num: '4', title: 'Задайте вопрос в чат', description: 'Уточните детали у ИИ-ассистента прямо в результатах' },
];

export function Landing() {
  const { isAuthenticated, isInitializing } = useAuthStore();

  if (isInitializing) {
    return null;
  }

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  return (
    <div className="min-h-screen bg-white">
      {/* Header */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 sm:h-16 flex items-center justify-between">
          <div className="flex items-center gap-4 sm:gap-6">
            <span className="text-lg sm:text-xl text-rose-600 shrink-0" style={{ fontFamily: "'Prosto One', cursive" }}>
              Умное сердце
            </span>
            <a
              href="#tools"
              className="hidden sm:inline-block text-sm text-gray-600 hover:text-gray-900 transition-colors py-2 whitespace-nowrap"
            >
              Калькуляторы
            </a>
          </div>
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
          <span className="inline-block text-sm font-medium text-rose-700 bg-rose-50 border border-rose-100 rounded-full px-4 py-1.5 mb-5">
            Для врачей, ординаторов и студентов-медиков
          </span>
          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 tracking-tight leading-tight">
            Анализ ЭКГ с помощью{' '}
            <span className="text-rose-600">искусственного интеллекта</span>
          </h1>
          <p className="mt-6 text-lg sm:text-xl text-gray-500 max-w-2xl mx-auto leading-relaxed">
            Загрузите фото электрокардиограммы и примерно за 30 секунд получите измерения
            по 12 отведениям, расчёт индексов гипертрофии и справочную интерпретацию
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
            3 бесплатных анализа всего, без привязки карты
          </p>
          <p className="mt-2 text-xs text-gray-400 max-w-2xl mx-auto">
            Сервис предназначен для информационной поддержки и не заменяет медицинское заключение врача.
          </p>
        </div>
      </section>

      {/* Trust strip */}
      <section className="pb-12 sm:pb-16 px-4 sm:px-6">
        <div className="max-w-4xl mx-auto grid grid-cols-1 sm:grid-cols-3 gap-4 sm:gap-6">
          {[
            {
              title: 'Обезличивание данных',
              description: 'ФИО и персональные данные пациента автоматически удаляются с изображения перед анализом',
              icon: (
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12c0 1.268-.63 2.39-1.593 3.068a3.745 3.745 0 0 1-1.043 3.296 3.745 3.745 0 0 1-3.296 1.043A3.745 3.745 0 0 1 12 21c-1.268 0-2.39-.63-3.068-1.593a3.746 3.746 0 0 1-3.296-1.043 3.745 3.745 0 0 1-1.043-3.296A3.745 3.745 0 0 1 3 12c0-1.268.63-2.39 1.593-3.068a3.745 3.745 0 0 1 1.043-3.296 3.746 3.746 0 0 1 3.296-1.043A3.746 3.746 0 0 1 12 3c1.268 0 2.39.63 3.068 1.593a3.746 3.746 0 0 1 3.296 1.043 3.746 3.746 0 0 1 1.043 3.296A3.745 3.745 0 0 1 21 12Z" />
              ),
            },
            {
              title: 'На основе клинических рекомендаций',
              description: 'Интерпретация и ответы чат-бота опираются на медицинскую литературу и клинические рекомендации',
              icon: (
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 0 0 6 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 0 1 6 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 0 1 6-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0 0 18 18a8.967 8.967 0 0 0-6 2.292m0-14.25v14.25" />
              ),
            },
            {
              title: '12 отведений и индексы ГЛЖ',
              description: 'Амплитуды, интервалы, ЧСС, ось QRS и расчёт индексов гипертрофии левого желудочка',
              icon: (
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 12h3l2.25 6 4.5-12 2.25 6h4.5" />
              ),
            },
          ].map((item) => (
            <div key={item.title} className="flex flex-col items-center text-center sm:items-start sm:text-left">
              <div className="w-10 h-10 rounded-lg bg-rose-100 text-rose-600 flex items-center justify-center mb-3">
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  {item.icon}
                </svg>
              </div>
              <h3 className="text-sm font-semibold text-gray-900">{item.title}</h3>
              <p className="text-sm text-gray-500 mt-1 leading-relaxed">{item.description}</p>
            </div>
          ))}
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

      {/* Free tools — no signup */}
      <section id="tools" className="scroll-mt-20 pb-16 sm:pb-20 px-4 sm:px-6">
        <div className="max-w-5xl mx-auto">
          <div className="text-center mb-8">
            <span className="inline-block text-xs font-medium text-rose-700 bg-rose-50 border border-rose-100 rounded-full px-3 py-1 mb-3">
              Бесплатно, без регистрации
            </span>
            <h2 className="text-2xl sm:text-3xl font-bold text-gray-900">Калькуляторы</h2>
            <p className="text-gray-500 mt-2">Попробуйте сервис без регистрации — клинические шкалы прямо в браузере</p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 sm:gap-6">
            {/* QTc */}
            <div className="bg-white border border-gray-100 rounded-2xl shadow-sm p-6 flex flex-col">
              <div className="w-12 h-12 rounded-xl bg-rose-50 border border-rose-100 flex items-center justify-center mb-4">
                <svg className="w-7 h-7 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 12h3l2.25 6 4.5-12 2.25 6h4.5" />
                </svg>
              </div>
              <h3 className="text-xl font-bold text-gray-900 mb-1.5">Калькулятор QTc</h3>
              <p className="text-sm text-gray-500 leading-relaxed mb-5 flex-1">
                Корригированный интервал QT по формулам Базетта, Фридерисии, Фрамингема и Ходжеса.
                Введите QT и ЧСС — RR подставится автоматически.
              </p>
              <Link
                to={ROUTES.QTC}
                className="inline-block self-start px-6 py-2.5 text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
              >
                Открыть
              </Link>
            </div>
            {/* Killip */}
            <div className="bg-white border border-gray-100 rounded-2xl shadow-sm p-6 flex flex-col">
              <div className="w-12 h-12 rounded-xl bg-rose-50 border border-rose-100 flex items-center justify-center mb-4">
                <svg className="w-7 h-7 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
                </svg>
              </div>
              <h3 className="text-xl font-bold text-gray-900 mb-1.5">Классификация по Killip</h3>
              <p className="text-sm text-gray-500 leading-relaxed mb-5 flex-1">
                Стратификация риска при остром инфаркте миокарда по признакам острой сердечной
                недостаточности с оценкой госпитальной летальности.
              </p>
              <Link
                to={ROUTES.KILLIP}
                className="inline-block self-start px-6 py-2.5 text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
              >
                Открыть
              </Link>
            </div>
            {/* CHA2DS2-VASc */}
            <div className="bg-white border border-gray-100 rounded-2xl shadow-sm p-6 flex flex-col">
              <div className="w-12 h-12 rounded-xl bg-rose-50 border border-rose-100 flex items-center justify-center mb-4">
                <svg className="w-7 h-7 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
                </svg>
              </div>
              <h3 className="text-xl font-bold text-gray-900 mb-1.5">Шкала CHA₂DS₂-VASc</h3>
              <p className="text-sm text-gray-500 leading-relaxed mb-5 flex-1">
                Оценка риска ишемического инсульта при фибрилляции предсердий и показаний
                к антикоагулянтной терапии.
              </p>
              <Link
                to={ROUTES.CHA2DS2VASC}
                className="inline-block self-start px-6 py-2.5 text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
              >
                Открыть
              </Link>
            </div>
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

      {/* Pricing */}
      <section className="py-16 sm:py-20 px-4 sm:px-6 bg-gray-50">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 text-center mb-12">
            Стоимость
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-5 max-w-3xl mx-auto items-stretch">
            {/* Free */}
            <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6 sm:p-8 flex flex-col">
              <p className="text-sm font-medium text-gray-500 uppercase tracking-wider mb-4">Бесплатно</p>
              <div className="flex items-baseline gap-1 mb-2">
                <span className="text-4xl sm:text-5xl font-bold text-gray-900">0</span>
                <span className="text-lg text-gray-500">&#8381;</span>
              </div>
              <p className="text-sm text-gray-500 mb-6">Чтобы попробовать сервис без оплаты</p>
              <ul className="text-sm text-gray-600 space-y-3 text-left mb-8 flex-1">
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  3 анализа ЭКГ
                </li>
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Все калькуляторы и шкалы
                </li>
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Без привязки карты
                </li>
              </ul>
              <Link
                to={ROUTES.REGISTER}
                className="block w-full px-6 py-3 text-base font-medium text-rose-700 bg-rose-50 hover:bg-rose-100 active:scale-95 rounded-xl transition-all duration-150 text-center"
              >
                Начать бесплатно
              </Link>
            </div>

            {/* Subscription */}
            <div className="relative bg-white rounded-2xl shadow-lg border-2 border-rose-500 p-6 sm:p-8 flex flex-col">
              <span className="absolute -top-3 left-1/2 -translate-x-1/2 text-xs font-semibold text-white bg-rose-600 rounded-full px-3 py-1">
                Полный доступ
              </span>
              <p className="text-sm font-medium text-gray-500 uppercase tracking-wider mb-4">Подписка</p>
              <div className="flex items-baseline gap-1 mb-2">
                <span className="text-4xl sm:text-5xl font-bold text-gray-900">1 990</span>
                <span className="text-lg text-gray-500">&#8381;/мес</span>
              </div>
              <p className="text-sm text-gray-500 mb-6">Доступ к сервису анализа ЭКГ на 1 месяц</p>
              <ul className="text-sm text-gray-600 space-y-3 text-left mb-8 flex-1">
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Безлимитные анализы ЭКГ
                </li>
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  Справочный чат-бот по кардиологии
                </li>
                <li className="flex items-start gap-2.5">
                  <svg className="w-5 h-5 text-rose-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                  </svg>
                  История всех анализов
                </li>
              </ul>
              <Link
                to={ROUTES.REGISTER}
                className="block w-full px-6 py-3 text-base font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200 text-center"
              >
                Попробовать бесплатно
              </Link>
            </div>
          </div>
          <p className="text-center text-xs text-gray-400 mt-5">
            Сначала 3 бесплатных анализа — подписка не требуется
          </p>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 sm:py-20 px-4 sm:px-6">
        <div className="max-w-2xl mx-auto text-center">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 mb-3">
            Начните прямо сейчас
          </h2>
          <p className="text-gray-500 mb-8">
            3 бесплатных анализа. Регистрация за 30 секунд.
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

      {/* Survey banner */}
      <section className="py-8 px-4 sm:px-6 bg-gray-50 border-y border-gray-100">
        <SurveyBanner className="max-w-3xl mx-auto" />
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-100 py-8 px-4 sm:px-6">
        <div className="max-w-6xl mx-auto flex flex-col items-center gap-4 text-xs text-gray-400 sm:flex-row sm:justify-between">
          <span className="text-center sm:text-left">Самозанятая Федутинова А.А., ИНН 575212369164</span>
          <div className="flex flex-wrap justify-center gap-x-4 gap-y-1">
            <Link to={ROUTES.QTC} className="hover:text-gray-600 transition-colors">Калькулятор QTc</Link>
            <Link to={ROUTES.KILLIP} className="hover:text-gray-600 transition-colors">Killip</Link>
            <Link to={ROUTES.CHA2DS2VASC} className="hover:text-gray-600 transition-colors">CHA₂DS₂-VASc</Link>
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
