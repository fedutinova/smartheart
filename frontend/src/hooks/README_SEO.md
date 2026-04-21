# Динамические мета-теги для SEO

## 📚 Структура

```
src/
├── hooks/
│   └── useMetaTags.ts          ← Основной хук для управления мета-тегами
├── config/
│   └── pageMeta.ts             ← Конфиг с информацией о всех страницах
│   └── pageMetaExamples.ts     ← Примеры использования
```

## 🚀 Быстрый старт

### 1. Использование в компоненте

```typescript
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function MyPage() {
  // Добавляем в начало компонента
  useMetaTags({
    ...getPageMeta('analyze'),              // Берем настройки из конфига
    canonical: getPageUrl('/analyze'),      // Указываем canonical URL
  });

  return (
    <div>Ваш контент</div>
  );
}
```

### 2. Для динамического контента

```typescript
export function ResultsPage({ results }: { results: ECGResults }) {
  useMetaTags({
    title: `Результаты анализа ЭКГ - ${results.date}`,
    description: `Анализ ЭКГ с детальной интерпретацией от ${results.date}`,
    ogImage: results.thumbnailUrl,
    canonical: getPageUrl(`/results/${results.id}`),
  });

  return <div>{/* контент */}</div>;
}
```

## 📖 API Reference

### useMetaTags(tags)

Хук для установки мета-тегов страницы.

**Параметры:**

```typescript
interface MetaTags {
  title: string;                    // Заголовок (отображается во вкладке)
  description: string;              // Описание (показывается в поиске)
  keywords?: string;                // Ключевые слова
  ogTitle?: string;                 // Заголовок для социальных сетей
  ogDescription?: string;           // Описание для соцсетей
  ogImage?: string;                 // Изображение для соцсетей
  canonical?: string;               // Canonical URL (для избежания дублей)
  robotsIndex?: 'index' | 'noindex';       // Индексировать ли страницу
  robotsFollow?: 'follow' | 'nofollow';    // Следить ли ссылки
}
```

**Пример:**

```typescript
useMetaTags({
  title: 'Анализ ЭКГ - Умное сердце',
  description: 'Загрузите фотографию ЭКГ и получите анализ за секунды',
  keywords: 'ЭКГ, анализ, ИИ',
  ogImage: 'https://smartheart.online/heart.svg',
  canonical: 'https://smartheart.online/analyze',
  robotsIndex: 'index',
  robotsFollow: 'follow',
});
```

### getPageMeta(pageName)

Получает предконфигурированные мета-теги для страницы.

**Параметры:**
- `pageName: string` - название страницы (см. ключи в `config/pageMeta.ts`)

**Возвращает:**
- `PageMetaConfig` - объект с title, description, keywords, и т.д.

**Пример:**

```typescript
const meta = getPageMeta('analyze');
// {
//   title: 'Анализ ЭКГ - Умное сердце',
//   description: '...',
//   keywords: '...',
// }
```

### getPageUrl(path)

Генерирует полный URL страницы для canonical и других целей.

**Параметры:**
- `path: string` - относительный путь (например, `/analyze`)

**Возвращает:**
- `string` - полный URL (например, `https://smartheart.online/analyze`)

**Пример:**

```typescript
const url = getPageUrl('/analyze');
// 'https://smartheart.online/analyze'
```

## 📝 Примеры использования

### Простая страница (Analyze)

```typescript
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Analyze() {
  useMetaTags({
    ...getPageMeta('analyze'),
    canonical: getPageUrl('/analyze'),
  });

  return <div>{/* контент */}</div>;
}
```

### С параметрами маршрута (Results)

```typescript
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';
import { useParams } from 'react-router-dom';

export function Results() {
  const { id } = useParams();

  useMetaTags({
    ...getPageMeta('results'),
    title: `Результаты анализа ЭКГ #${id}`,
    canonical: getPageUrl(`/results/${id}`),
  });

  return <div>{/* контент */}</div>;
}
```

### С запросом API (Article)

```typescript
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageUrl } from '@/config/pageMeta';
import { useQuery } from '@tanstack/react-query';

export function Article() {
  const { id } = useParams();
  const { data: article } = useQuery(/* ... */);

  useMetaTags({
    title: article?.title || 'Статья',
    description: article?.excerpt || '',
    ogImage: article?.featuredImage,
    canonical: getPageUrl(`/articles/${id}`),
  });

  return <div>{/* контент */}</div>;
}
```

### Со скрытием от поиска (Login)

```typescript
import { useMetaTags } from '@/hooks/useMetaTags';
import { getPageMeta, getPageUrl } from '@/config/pageMeta';

export function Login() {
  useMetaTags({
    ...getPageMeta('login'),
    canonical: getPageUrl('/login'),
    robotsIndex: 'noindex',       // Не индексировать
    robotsFollow: 'nofollow',      // Не следить ссылки
  });

  return <div>{/* контент */}</div>;
}
```

## 🔍 Что происходит при вызове

Хук `useMetaTags()` автоматически:

1. ✅ Обновляет `document.title` → видно во вкладке браузера
2. ✅ Обновляет `<meta name="description">` → показывается в поиске Google
3. ✅ Обновляет Open Graph теги → красивая карточка в соцсетях
4. ✅ Обновляет Twitter Card → красивая карточка в Twitter/X
5. ✅ Обновляет Canonical link → защита от дублей контента
6. ✅ Обновляет robots директивы → контроль индексирования

## 🐛 Отладка

Если мета-теги не обновляются:

1. Откройте DevTools (F12)
2. Перейдите на страницу
3. В консоли выполните:
   ```javascript
   document.title  // Должен быть новый title
   document.querySelector('meta[name="description"]').content  // Должно быть новое description
   ```
4. Проверьте, что `useMetaTags` вызывается в начале компонента

## ✅ Чек-лист для внедрения

- [ ] Добавить `useMetaTags()` в Analyze.tsx
- [ ] Добавить `useMetaTags()` в Results.tsx
- [ ] Добавить `useMetaTags()` в History.tsx
- [ ] Добавить `useMetaTags()` в Dashboard.tsx
- [ ] Добавить `useMetaTags()` в Account.tsx
- [ ] Добавить `useMetaTags()` в KnowledgeBase.tsx
- [ ] Добавить `useMetaTags()` в Login.tsx (с noindex)
- [ ] Добавить `useMetaTags()` в Register.tsx (с noindex)
- [ ] Добавить `useMetaTags()` в ForgotPassword.tsx (с noindex)
- [ ] Добавить `useMetaTags()` в Contacts.tsx
- [ ] Добавить `useMetaTags()` в Privacy.tsx (с noindex)
- [ ] Добавить `useMetaTags()` в Terms.tsx (с noindex)

## 📚 Дополнительные ресурсы

- [React Hooks Best Practices](https://react.dev/reference/react/useEffect)
- [Open Graph Protocol](https://ogp.me/)
- [Twitter Card Documentation](https://developer.twitter.com/en/docs/twitter-for-websites/cards/overview/abouts-cards)
- [Canonical Links - Google Search Central](https://developers.google.com/search/docs/crawling-indexing/canonicalization)
