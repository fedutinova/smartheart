import { Link } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { Layout } from '@/components/Layout';
import { ROUTES } from '@/config';

const teamMembers = [
  {
    name: 'Алимова Елена',
    role: 'Врач функциональной диагностики',
    responsibilities: ['Работа с ЭКГ', 'Дата-аналитик', 'NLP-специалист'],
    initials: 'АЕ',
  },
  {
    name: 'Федутинова Анна',
    role: 'Фулстек разработчик',
    responsibilities: ['Разработка фронтенда', 'Разработка бэкенда'],
    initials: 'ФА',
  },
];

function ContactsContent() {
  return (
    <div className="max-w-3xl mx-auto">
      <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 mb-2">О нас</h1>
      <p className="text-gray-500 mb-10 leading-relaxed">
        <span style={{ fontFamily: "'Prosto One', cursive" }} className="text-gray-700">Умное сердце</span>, платформа
        для анализа электрокардиограмм с помощью искусственного интеллекта.
        Объединяем медицинскую экспертизу и разработку ПО для помощи в диагностике
        сердечно-сосудистых заболеваний.
      </p>

      <h2 className="text-lg font-semibold text-gray-900 mb-6">Команда</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-12">
        {teamMembers.map((member) => (
          <div
            key={member.name}
            className="p-5 rounded-2xl border border-gray-100 bg-white"
          >
            <div className="flex items-center gap-3 mb-3">
              <div className="flex-shrink-0 w-11 h-11 rounded-full bg-gradient-to-br from-rose-400 to-purple-400 flex items-center justify-center text-white text-sm font-semibold">
                {member.initials}
              </div>
              <div>
                <h3 className="text-base font-semibold text-gray-900">{member.name}</h3>
                <p className="text-sm text-rose-500">{member.role}</p>
              </div>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {member.responsibilities.map((r) => (
                <span
                  key={r}
                  className="px-2.5 py-0.5 text-xs text-gray-500 bg-gray-100 rounded-full"
                >
                  {r}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>

      <h2 className="text-lg font-semibold text-gray-900 mb-4">Контакты</h2>
      <div className="p-5 rounded-2xl border border-gray-100 bg-white space-y-3">
        <div className="flex items-center gap-3">
          <svg className="w-5 h-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" />
          </svg>
          <a href="mailto:support@smartheart.cloud" className="text-sm text-rose-600 hover:text-rose-700 transition-colors">
            support@smartheart.cloud
          </a>
        </div>
        <div className="flex items-center gap-3">
          <svg className="w-5 h-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 9h3.75M15 12h3.75M15 15h3.75M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Zm6-10.125a1.875 1.875 0 1 1-3.75 0 1.875 1.875 0 0 1 3.75 0Zm1.294 6.336a6.721 6.721 0 0 1-3.17.789 6.721 6.721 0 0 1-3.168-.789 3.376 3.376 0 0 1 6.338 0Z" />
          </svg>
          <span className="text-sm text-gray-600">Федутинова Анна Александровна, ИНН 575212369164</span>
        </div>
      </div>

      <div className="mt-8 pt-6 border-t border-gray-100 flex flex-wrap gap-4 text-xs text-gray-400">
        <Link to={ROUTES.TERMS} className="hover:text-gray-600 transition-colors">Оферта</Link>
        <Link to={ROUTES.PRIVACY} className="hover:text-gray-600 transition-colors">Конфиденциальность</Link>
      </div>
    </div>
  );
}

export function Contacts() {
  const { isAuthenticated } = useAuthStore();

  if (isAuthenticated) {
    return (
      <Layout>
        <ContactsContent />
      </Layout>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
          <Link
            to={ROUTES.HOME}
            className="text-xl text-rose-600 hover:text-rose-700 transition-colors"
            style={{ fontFamily: "'Prosto One', cursive" }}
          >
            Умное сердце
          </Link>
          <div className="flex items-center gap-3">
            <Link to={ROUTES.LOGIN} className="text-sm text-gray-600 hover:text-gray-900 transition-colors px-3 py-2">
              Войти
            </Link>
            <Link to={ROUTES.REGISTER} className="text-sm font-medium text-white bg-rose-600 hover:bg-rose-700 px-4 py-2 rounded-lg transition-colors">
              Регистрация
            </Link>
          </div>
        </div>
      </header>
      <main className="max-w-6xl mx-auto px-4 sm:px-6 py-10 sm:py-16">
        <ContactsContent />
      </main>
    </div>
  );
}
