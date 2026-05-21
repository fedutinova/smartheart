import { useState } from 'react';

const STORAGE_KEY = 'survey_banner_dismissed';
const SURVEY_URL =
  'https://docs.google.com/forms/d/e/1FAIpQLSerfoqa5aM-7A-Yei7gZxs8CxGgmBu_dlMx4XqolAijSTbFBg/viewform';

export function SurveyBanner({ className = '' }: { className?: string }) {
  const [dismissed, setDismissed] = useState(
    () => localStorage.getItem(STORAGE_KEY) === '1',
  );

  if (dismissed) return null;

  const dismiss = () => {
    localStorage.setItem(STORAGE_KEY, '1');
    setDismissed(true);
  };

  return (
    <div
      className={`rounded-xl bg-gradient-to-r from-indigo-600 to-purple-600 px-5 py-4 flex items-center gap-3 ${className}`}
    >
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-white">Помогите улучшить сервис</p>
        <p className="text-sm text-indigo-100 mt-0.5">
          Пройдите короткий опрос о вашем опыте — займёт 2–3 минуты
        </p>
      </div>
      <a
        href={SURVEY_URL}
        target="_blank"
        rel="noopener noreferrer"
        className="shrink-0 hidden sm:inline-block px-4 py-2 text-sm font-medium text-indigo-700 bg-white hover:bg-indigo-50 rounded-lg transition-colors"
      >
        Пройти опрос →
      </a>
      <a
        href={SURVEY_URL}
        target="_blank"
        rel="noopener noreferrer"
        className="shrink-0 sm:hidden text-sm font-medium text-white underline underline-offset-2"
      >
        Пройти →
      </a>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Закрыть баннер"
        className="shrink-0 p-1 rounded-full text-indigo-200 hover:text-white hover:bg-white/20 transition-colors"
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  );
}
