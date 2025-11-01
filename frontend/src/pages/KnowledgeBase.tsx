import { Layout } from '@/components/Layout';
import { useState } from 'react';

interface KnowledgeArticle {
  id: string;
  title: string;
  category: string;
  description: string;
  icon: string;
}

const categories = ['Все', 'ЭКГ', 'Диагностика', 'Симптомы', 'Лечение'];

const mockArticles: KnowledgeArticle[] = [
  {
    id: '1',
    title: 'Основы электрокардиографии',
    category: 'ЭКГ',
    description: 'Изучите основы чтения и интерпретации электрокардиограмм',
    icon: '📊',
  },
  {
    id: '2',
    title: 'Нормальные показатели ЭКГ',
    category: 'ЭКГ',
    description: 'Какие значения считаются нормальными для здорового сердца',
    icon: '✅',
  },
  {
    id: '3',
    title: 'Аритмии: виды и признаки',
    category: 'Диагностика',
    description: 'Различные типы нарушений сердечного ритма и их проявления',
    icon: '💓',
  },
  {
    id: '4',
    title: 'Ишемическая болезнь сердца',
    category: 'Диагностика',
    description: 'Признаки ишемии на электрокардиограмме',
    icon: '🩺',
  },
  {
    id: '5',
    title: 'Боль в груди: когда обращаться к врачу',
    category: 'Симптомы',
    description: 'Важные признаки, требующие немедленной медицинской помощи',
    icon: '⚠️',
  },
  {
    id: '6',
    title: 'Одышка и сердечные заболевания',
    category: 'Симптомы',
    description: 'Как одышка может указывать на проблемы с сердцем',
    icon: '😮‍💨',
  },
  {
    id: '7',
    title: 'Медикаментозное лечение аритмий',
    category: 'Лечение',
    description: 'Обзор препаратов для лечения нарушений ритма',
    icon: '💊',
  },
  {
    id: '8',
    title: 'Профилактика сердечно-сосудистых заболеваний',
    category: 'Лечение',
    description: 'Образ жизни и привычки для здорового сердца',
    icon: '❤️',
  },
];

export function KnowledgeBase() {
  const [selectedCategory, setSelectedCategory] = useState('Все');
  const [searchQuery, setSearchQuery] = useState('');

  const filteredArticles = mockArticles.filter((article) => {
    const matchesCategory =
      selectedCategory === 'Все' || article.category === selectedCategory;
    const matchesSearch =
      searchQuery === '' ||
      article.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      article.description.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesCategory && matchesSearch;
  });

  return (
    <Layout>
      <div className="px-4 sm:px-0">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            База знаний
          </h1>
          <p className="text-gray-600">
            Полезная информация о здоровье сердца и электрокардиографии
          </p>
        </div>

        {/* Search */}
        <div className="mb-6">
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
              <span className="text-gray-400 text-xl">🔍</span>
            </div>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Поиск по базе знаний..."
              className="block w-full pl-10 pr-3 py-3 border border-gray-300 rounded-lg leading-5 bg-white placeholder-gray-500 focus:outline-none focus:placeholder-gray-400 focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>
        </div>

        {/* Categories */}
        <div className="mb-6 flex flex-wrap gap-2">
          {categories.map((category) => (
            <button
              key={category}
              onClick={() => setSelectedCategory(category)}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                selectedCategory === category
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-gray-700 hover:bg-gray-100 border border-gray-300'
              }`}
            >
              {category}
            </button>
          ))}
        </div>

        {/* Articles Grid */}
        {filteredArticles.length === 0 ? (
          <div className="bg-white rounded-lg shadow p-12 text-center">
            <p className="text-gray-500 text-lg">
              Статьи не найдены. Попробуйте изменить параметры поиска.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {filteredArticles.map((article) => (
              <div
                key={article.id}
                className="bg-white rounded-lg shadow hover:shadow-lg transition-shadow p-6 cursor-pointer border border-gray-200 hover:border-blue-300"
              >
                <div className="flex items-start">
                  <div className="text-4xl mr-4">{article.icon}</div>
                  <div className="flex-1">
                    <div className="mb-2">
                      <span className="inline-block px-2 py-1 text-xs font-semibold rounded bg-blue-100 text-blue-800">
                        {article.category}
                      </span>
                    </div>
                    <h3 className="text-xl font-semibold text-gray-900 mb-2">
                      {article.title}
                    </h3>
                    <p className="text-gray-600 text-sm">{article.description}</p>
                    <div className="mt-4">
                      <button className="text-blue-600 hover:text-blue-800 font-medium text-sm">
                        Читать далее →
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Info Message */}
        <div className="mt-8 bg-blue-50 border border-blue-200 rounded-lg p-6">
          <div className="flex">
            <div className="flex-shrink-0">
              <span className="text-2xl">ℹ️</span>
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-blue-800">
                Информация
              </h3>
              <p className="mt-1 text-sm text-blue-700">
                База знаний находится в разработке. Содержимое будет пополняться
                новыми статьями и материалами.
              </p>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}

