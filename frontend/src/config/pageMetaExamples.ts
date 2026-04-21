/**
 * ПРИМЕРЫ использования хука useMetaTags в компонентах страниц
 *
 * Скопируйте эти примеры в начало функций компонентов
 */

// ============================================
// 1. Анализ (Analyze.tsx)
// ============================================
export const analyzePageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Analyze() {
  // Установить мета-теги для этой страницы
  useMetaTags({
    ...getPageMeta('analyze'),
    canonical: getPageUrl('/analyze'),
  });

  // ... rest of component
}
`;

// ============================================
// 2. История анализов (History.tsx)
// ============================================
export const historyPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function History() {
  useMetaTags({
    ...getPageMeta('history'),
    canonical: getPageUrl('/history'),
  });

  // ... rest of component
}
`;

// ============================================
// 3. Результаты (Results.tsx)
// ============================================
export const resultsPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Results() {
  const { id } = useParams();

  useMetaTags({
    ...getPageMeta('results'),
    // Можно переопределить для более специфичного контента
    title: 'Результаты анализа ЭКГ - Умное сердце',
    description: 'Детальный анализ вашей электрокардиограммы с интерпретацией показателей.',
    canonical: getPageUrl(\`/results/\${id}\`),
  });

  // ... rest of component
}
`;

// ============================================
// 4. Панель управления (Dashboard.tsx)
// ============================================
export const dashboardPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Dashboard() {
  useMetaTags({
    ...getPageMeta('dashboard'),
    canonical: getPageUrl('/dashboard'),
  });

  // ... rest of component
}
`;

// ============================================
// 5. Аккаунт (Account.tsx)
// ============================================
export const accountPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Account() {
  useMetaTags({
    ...getPageMeta('account'),
    canonical: getPageUrl('/account'),
  });

  // ... rest of component
}
`;

// ============================================
// 6. База знаний (KnowledgeBase.tsx)
// ============================================
export const knowledgeBaseExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function KnowledgeBase() {
  useMetaTags({
    ...getPageMeta('knowledgeBase'),
    canonical: getPageUrl('/knowledge-base'),
  });

  // ... rest of component
}
`;

// ============================================
// 7. Авторизация (Login.tsx)
// ============================================
export const loginPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Login() {
  useMetaTags({
    ...getPageMeta('login'),
    canonical: getPageUrl('/login'),
    robotsIndex: 'noindex', // Скрыть от поиска (внутренняя страница)
    robotsFollow: 'nofollow',
  });

  // ... rest of component
}
`;

// ============================================
// 8. Регистрация (Register.tsx)
// ============================================
export const registerPageExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Register() {
  useMetaTags({
    ...getPageMeta('register'),
    canonical: getPageUrl('/register'),
    robotsIndex: 'noindex',
    robotsFollow: 'nofollow',
  });

  // ... rest of component
}
`;

// ============================================
// Динамический контент (пример для стать)
// ============================================
export const dynamicContentExample = `
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageUrl } from '@/config/pageMeta';

interface Article {
  title: string;
  description: string;
  imageUrl?: string;
  publishDate: string;
  author: string;
}

export function ArticlePage({ article }: { article: Article }) {
  // Для динамического контента полностью переопределяем мета-теги
  useMetaTags({
    title: \`\${article.title} - Умное сердце\`,
    description: article.description,
    keywords: article.title + ', ЭКГ, здоровье',
    ogTitle: article.title,
    ogDescription: article.description,
    ogImage: article.imageUrl,
    canonical: getPageUrl(\`/articles/\${article.id}\`),
  });

  return (
    // Компонент статьи
  );
}
`;
