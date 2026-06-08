import { Link } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { KillipWidget } from '@/components/KillipWidget';
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';
import { useAuthStore } from '@/store/auth';
import { ROUTES } from '@/config';

function KillipHeader() {
  return (
    <div className="flex items-center gap-3 mb-4 sm:mb-6">
      <div className="w-9 h-9 rounded-full bg-gradient-to-br from-rose-500 to-rose-600 flex items-center justify-center shrink-0">
        <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
        </svg>
      </div>
      <div>
        <h1 className="text-lg font-bold text-gray-900">Классификация по Killip</h1>
        <p className="text-xs text-gray-500">Стратификация риска при остром инфаркте миокарда по признакам острой СН</p>
      </div>
    </div>
  );
}

export function Killip() {
  const { isAuthenticated, isInitializing } = useAuthStore();
  const meta = getPageMeta('killip');
  useMetaTags({
    title: meta.title,
    description: meta.description,
    keywords: meta.keywords,
    ogDescription: meta.ogDescription,
    canonical: getPageUrl('/killip'),
  });

  // Пока определяется статус авторизации — не мигаем гостевой оболочкой
  if (isInitializing) {
    return null;
  }

  // Авторизованным — обычная оболочка приложения с навигацией
  if (isAuthenticated) {
    return (
      <Layout>
        <div className="max-w-3xl mx-auto">
          <KillipHeader />
          <KillipWidget />
        </div>
      </Layout>
    );
  }

  // Гостям — публичная оболочка в стиле лендинга, без регистрации
  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <header className="bg-white/80 backdrop-blur-md border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 sm:h-16 flex items-center justify-between">
          <Link to={ROUTES.HOME} className="text-lg sm:text-xl text-rose-600 shrink-0" style={{ fontFamily: "'Prosto One', cursive" }}>
            Умное сердце
          </Link>
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

      <main className="flex-1 w-full max-w-3xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
        <KillipHeader />
        <KillipWidget />

        {/* CTA — переход к основному сервису */}
        <div className="mt-6 bg-white rounded-lg border border-gray-200 p-5 text-center">
          <p className="text-sm text-gray-700 mb-1">Анализируете ЭКГ?</p>
          <p className="text-xs text-gray-500 mb-4">
            Загрузите фото плёнки и получите измерения по 12 отведениям, индексы ГЛЖ и справочную интерпретацию
          </p>
          <Link
            to={ROUTES.REGISTER}
            className="inline-block px-6 py-2.5 text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 active:scale-95 rounded-xl transition-all duration-150 shadow-lg shadow-rose-200"
          >
            Попробовать бесплатно
          </Link>
          <p className="mt-2 text-xs text-gray-400">3 бесплатных анализа, без привязки карты</p>
        </div>
      </main>

      <footer className="border-t border-gray-100 py-6 px-4 sm:px-6">
        <p className="max-w-6xl mx-auto text-center text-[11px] text-gray-300 leading-relaxed">
          Сервис не является медицинским изделием. Результаты носят информационный характер и не заменяют консультацию врача.
        </p>
      </footer>
    </div>
  );
}
