import { Link } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { ROUTES } from '@/config';

export function Privacy() {
  return (
    <Layout>
      <div className="max-w-4xl mx-auto py-8 px-4 sm:px-6">
        <button
          onClick={() => window.history.back()}
          className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 transition-colors mb-4"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
          </svg>
          Назад
        </button>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Политика конфиденциальности</h1>

        <div className="prose prose-sm prose-gray max-w-none space-y-6 text-gray-700">
          <p className="text-sm text-gray-400">Дата публикации: 22 марта 2026 г.</p>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">1. Общие положения</h2>
            <p>
              Настоящая Политика конфиденциальности (далее «Политика») определяет порядок обработки
              и защиты персональных данных пользователей сервиса «Умное сердце» (далее «Сервис»).
            </p>
            <p>
              Оператор персональных данных: самозанятая Федутинова Анна Александровна (ИНН: 575212369164),
              плательщик налога на профессиональный доход (422-ФЗ).
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">2. Какие данные мы собираем</h2>
            <ul className="list-disc pl-5 space-y-1">
              <li>Имя пользователя, адрес электронной почты: при регистрации</li>
              <li>Изображения ЭКГ и примечания: при отправке на анализ</li>
              <li>Текстовые запросы: при использовании базы знаний</li>
              <li>Данные об оплате (ID транзакции, сумма): при покупке анализов</li>
              <li>Техническая информация (IP-адрес, User-Agent): автоматически</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">3. Цели обработки</h2>
            <ul className="list-disc pl-5 space-y-1">
              <li>Предоставление услуг Сервиса (анализ ЭКГ, Чат-бот)</li>
              <li>Идентификация и аутентификация пользователя</li>
              <li>Обработка платежей и формирование чеков</li>
              <li>Улучшение качества Сервиса</li>
              <li>Связь с пользователем по техническим вопросам</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">4. Хранение и защита данных</h2>
            <p>
              Персональные данные хранятся на серверах с использованием шифрования.
              Пароли хранятся в хэшированном виде (bcrypt). Доступ к данным ограничен.
            </p>
            <p>
              Изображения ЭКГ хранятся в зашифрованном объектном хранилище и автоматически удаляются
              через 90 дней после анализа.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">5. Передача данных третьим лицам</h2>
            <p>Данные могут передаваться:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>OpenAI: изображения ЭКГ для проведения анализа (обезличено)</li>
              <li>ЮKassa: данные для обработки платежей</li>
            </ul>
            <p>Данные не продаются и не передаются иным третьим лицам.</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">6. Права пользователя</h2>
            <p>Вы имеете право:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Запросить информацию о хранимых персональных данных</li>
              <li>Потребовать исправления или удаления персональных данных</li>
              <li>Отозвать согласие на обработку персональных данных</li>
            </ul>
            <p>
              Для реализации прав направьте запрос на email: <strong>support@smartheart.cloud</strong>
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">7. Файлы cookie</h2>
            <p>
              Сервис использует localStorage и sessionStorage для хранения токенов авторизации
              и пользовательских настроек. Сторонние cookie не используются.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">8. Изменения политики</h2>
            <p>
              Оператор оставляет за собой право вносить изменения в Политику. Актуальная версия
              всегда доступна по адресу{' '}
              <Link to={ROUTES.PRIVACY} className="text-rose-600 hover:text-rose-500">
                smartheart.cloud/privacy
              </Link>.
            </p>
          </section>
        </div>
      </div>
    </Layout>
  );
}
