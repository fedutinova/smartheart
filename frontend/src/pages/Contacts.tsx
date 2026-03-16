import { Layout } from '@/components/Layout';

interface TeamMember {
  name: string;
  role: string;
  responsibilities: string[];
  icon: string;
}

const teamMembers: TeamMember[] = [
  {
    name: 'Алимова Елена',
    role: 'Врач ФД',
    responsibilities: ['Работа с ЭКГ', 'Дата-аналитик', 'NLP-специалист'],
    icon: '👩‍⚕️',
  },
  {
    name: 'Федутинова Анна',
    role: 'Фулстек разработчик',
    responsibilities: ['Разработка фронтенда', 'Разработка бэкенда'],
    icon: '👩‍💻',
  },
];

export function Contacts() {
  return (
    <Layout>
      <div className="px-4 sm:px-0">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 mb-2">Контакты</h1>
          <p className="text-gray-600">
            Наша команда разработки проекта Умное сердце
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {teamMembers.map((member, index) => (
            <div
              key={index}
              className="bg-white rounded-lg shadow-lg hover:shadow-xl transition-shadow p-6 border border-gray-200"
            >
              <div className="text-center mb-4">
                <div className="text-6xl mb-4">{member.icon}</div>
                <h3 className="text-xl font-bold text-gray-900 mb-1">
                  {member.name}
                </h3>
                <p className="text-sm font-medium text-blue-600 mb-4">
                  {member.role}
                </p>
              </div>
              <div className="border-t border-gray-200 pt-4">
                <h4 className="text-sm font-semibold text-gray-700 mb-3">
                  Направления работы:
                </h4>
                <ul className="space-y-2">
                  {member.responsibilities.map((responsibility, idx) => (
                    <li
                      key={idx}
                      className="flex items-center text-sm text-gray-600"
                    >
                      <span className="text-blue-500 mr-2">•</span>
                      {responsibility}
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          ))}
        </div>

        {/* Additional Info Section */}
        <div className="mt-8 bg-gradient-to-br from-blue-50 to-purple-50 border border-blue-200 rounded-lg p-6">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <span className="text-2xl">💡</span>
            </div>
            <div className="ml-3">
              <h3 className="text-lg font-semibold text-gray-900 mb-2">
                О проекте Умное сердце
              </h3>
              <p className="text-sm text-gray-700">
                Умное сердце — это инновационная платформа для анализа
                электрокардиограмм с использованием технологий машинного обучения
                и обработки естественного языка. Наша команда объединяет экспертизу
                в области медицины, разработки программного обеспечения и анализа
                данных для создания инструментов, помогающих в диагностике
                сердечно-сосудистых заболеваний.
              </p>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}



