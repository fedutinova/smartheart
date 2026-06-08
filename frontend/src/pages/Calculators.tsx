import { Link } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';
import { ROUTES } from '@/config';

const CALCULATORS = [
  {
    to: ROUTES.QTC,
    title: 'Калькулятор QTc',
    description: 'Корригированный интервал QT по формулам Базетта, Фридерисии, Фрамингема и Ходжеса',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 12h3l2.25 6 4.5-12 2.25 6h4.5" />
    ),
  },
  {
    to: ROUTES.KILLIP,
    title: 'Классификация по Killip',
    description: 'Стратификация риска при остром инфаркте миокарда с оценкой госпитальной летальности',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
    ),
  },
  {
    to: ROUTES.CHA2DS2VASC,
    title: 'Шкала CHA₂DS₂-VASc',
    description: 'Риск ишемического инсульта при фибрилляции предсердий и показания к антикоагулянтной терапии',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
    ),
  },
];

export function Calculators() {
  const meta = getPageMeta('calculators');
  useMetaTags({
    title: meta.title,
    description: meta.description,
    keywords: meta.keywords,
    ogDescription: meta.ogDescription,
    canonical: getPageUrl('/calculators'),
  });

  return (
    <Layout>
      <div className="max-w-3xl mx-auto">
        <div className="mb-4 sm:mb-6">
          <h1 className="text-lg font-bold text-gray-900">Калькуляторы</h1>
          <p className="text-xs text-gray-500">Клинические шкалы и калькуляторы для кардиологии</p>
        </div>

        <div className="space-y-3">
          {CALCULATORS.map((c) => (
            <Link
              key={c.to}
              to={c.to}
              className="flex items-center gap-4 bg-white rounded-lg shadow-sm border border-gray-100 p-4 hover:border-rose-200 hover:shadow active:scale-[0.99] transition-all"
            >
              <div className="w-11 h-11 rounded-xl bg-rose-50 border border-rose-100 flex items-center justify-center shrink-0">
                <svg className="w-6 h-6 text-rose-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.6}>
                  {c.icon}
                </svg>
              </div>
              <div className="min-w-0 flex-1">
                <h2 className="text-sm font-semibold text-gray-900">{c.title}</h2>
                <p className="text-xs text-gray-500 mt-0.5 leading-relaxed">{c.description}</p>
              </div>
              <svg className="w-5 h-5 text-gray-300 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
              </svg>
            </Link>
          ))}
        </div>
      </div>
    </Layout>
  );
}
