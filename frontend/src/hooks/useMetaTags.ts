import { useEffect } from 'react';

interface MetaTags {
  title: string;
  description: string;
  keywords?: string;
  ogTitle?: string;
  ogDescription?: string;
  ogImage?: string;
  canonical?: string;
  robotsIndex?: 'index' | 'noindex';
  robotsFollow?: 'follow' | 'nofollow';
}

/**
 * Hook для управления динамическими мета-тегами для SEO
 * Обновляет заголовок страницы, meta description, Open Graph и другие теги
 */
export function useMetaTags(tags: MetaTags) {
  useEffect(() => {
    // Title
    document.title = tags.title;
    updateMetaTag('og:title', tags.ogTitle || tags.title);
    updateMetaTag('twitter:title', tags.ogTitle || tags.title);

    // Description
    updateMetaTag('description', tags.description);
    updateMetaTag('og:description', tags.ogDescription || tags.description);
    updateMetaTag('twitter:description', tags.ogDescription || tags.description);

    // Keywords
    if (tags.keywords) {
      updateMetaTag('keywords', tags.keywords);
    }

    // Open Graph Image
    if (tags.ogImage) {
      updateMetaTag('og:image', tags.ogImage);
      updateMetaTag('twitter:image', tags.ogImage);
    }

    // Canonical URL
    if (tags.canonical) {
      updateCanonicalLink(tags.canonical);
    }

    // Robots directives
    if (tags.robotsIndex || tags.robotsFollow) {
      const robotsContent = `${tags.robotsIndex || 'index'}, ${tags.robotsFollow || 'follow'}`;
      updateMetaTag('robots', robotsContent);
    }

    // Return cleanup function (optional)
    return () => {
      // Можно добавить очистку если нужно
    };
  }, [tags]);
}

/**
 * Вспомогательная функция для обновления или создания мета-тега
 */
function updateMetaTag(name: string, content: string) {
  let element = document.querySelector(`meta[name="${name}"], meta[property="${name}"]`);

  if (!element) {
    element = document.createElement('meta');
    if (name.startsWith('og:') || name.startsWith('twitter:')) {
      element.setAttribute('property', name);
    } else {
      element.setAttribute('name', name);
    }
    document.head.appendChild(element);
  }

  element.setAttribute('content', content);
}

/**
 * Обновляет или создает canonical link
 */
function updateCanonicalLink(url: string) {
  let link = document.querySelector('link[rel="canonical"]');

  if (!link) {
    link = document.createElement('link');
    link.setAttribute('rel', 'canonical');
    document.head.appendChild(link);
  }

  link.setAttribute('href', url);
}
