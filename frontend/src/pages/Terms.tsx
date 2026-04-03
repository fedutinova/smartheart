import { Link } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { ROUTES } from '@/config';

export function Terms() {
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
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Публичная оферта</h1>

        <div className="prose prose-sm prose-gray max-w-none space-y-6 text-gray-700">
          <p className="text-sm text-gray-400">Дата публикации: 22 марта 2026 г.</p>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">1. Общие положения</h2>
            <p>
              Настоящий документ является публичной офертой (далее «Оферта») самозанятого
              Федутиновой Анны Александровны (ИНН: 575212369164), плательщика налога на профессиональный доход
              в соответствии с Федеральным законом № 422-ФЗ (далее «Исполнитель»), и определяет
              условия предоставления услуг сервиса «Умное сердце» (далее «Сервис»).
            </p>
            <p>
              Регистрация в Сервисе и/или оплата услуг является полным и безоговорочным
              акцептом настоящей Оферты (ст. 438 ГК РФ).
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">2. Предмет оферты</h2>
            <p>
              Исполнитель предоставляет Пользователю доступ к Сервису для автоматизированной
              обработки изображений электрокардиограмм (ЭКГ), получения справочной интерпретации
              и использования базы знаний по кардиологии.
            </p>
            <p>
              Сервис предназначен для информационной поддержки врача и пользователя, но не для
              самостоятельной постановки диагноза и не для оказания медицинской помощи.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">3. Стоимость и порядок оплаты</h2>
            <ul className="list-disc pl-5 space-y-1">
              <li>Сервис предоставляет 2 (два) бесплатных анализа ЭКГ в сутки</li>
              <li>Дополнительные анализы приобретаются пакетами (1, 5 или 10 анализов)</li>
              <li>Актуальная стоимость отображается на странице оплаты</li>
              <li>Оплата осуществляется через платёжный сервис ЮKassa</li>
              <li>За каждую оплату формируется чек в соответствии с 422-ФЗ</li>
              <li>Оплаченные анализы не имеют срока давности</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">4. Порядок возврата</h2>
            <p>
              Возврат средств возможен в случае, если оплаченные анализы не были использованы.
              Для оформления возврата направьте запрос на email: <strong>support@smartheart.online</strong>
              с указанием ID платежа. Срок рассмотрения: до 10 рабочих дней.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">5. Медицинский дисклеймер</h2>
            <p className="font-medium text-gray-900">
              Сервис «Умное сердце» НЕ является медицинским изделием и НЕ предназначен для
              постановки медицинских диагнозов.
            </p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Результаты анализа носят исключительно информационный и справочный характер</li>
              <li>Ответы чат-бота формируются автоматически и должны проверяться пользователем</li>
              <li>Результаты не заменяют консультацию квалифицированного врача</li>
              <li>Не используйте результаты Сервиса для самодиагностики или назначения лечения</li>
              <li>При подозрении на сердечно-сосудистое заболевание обратитесь к врачу</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">6. Ограничение ответственности</h2>
            <p>
              Исполнитель не несёт ответственности за решения, принятые на основании результатов
              работы Сервиса. Исполнитель прилагает
              разумные усилия для обеспечения точности анализа, но не гарантирует его
              безошибочность.
            </p>
            <p>
              Пользователь обязуется не передавать в Сервис прямые идентификаторы пациента без
              самостоятельного правового основания на такую передачу и, по возможности, использовать
              обезличенные данные.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">7. Персональные данные</h2>
            <p>
              Обработка персональных данных осуществляется в соответствии с{' '}
              <Link to={ROUTES.PRIVACY} className="text-rose-600 hover:text-rose-500">
                Политикой конфиденциальности
              </Link>.
            </p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">8. Информация об исполнителе</h2>
            <ul className="list-none space-y-1">
              <li><strong>ФИО:</strong> Федутинова Анна Александровна</li>
              <li><strong>Статус:</strong> Самозанятый, плательщик НПД (422-ФЗ)</li>
              <li><strong>ИНН:</strong> 575212369164</li>
              <li><strong>Email:</strong> support@smartheart.online</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-gray-900">9. Изменение условий</h2>
            <p>
              Исполнитель вправе в одностороннем порядке изменять условия Оферты. Актуальная
              версия доступна по адресу{' '}
              <Link to={ROUTES.TERMS} className="text-rose-600 hover:text-rose-500">
                smartheart.online/terms
              </Link>.
              Продолжение использования Сервиса после изменений означает согласие с новой
              редакцией Оферты.
            </p>
          </section>
        </div>
      </div>
    </Layout>
  );
}
