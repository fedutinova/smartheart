import { Layout } from '@/components/Layout';

interface TeamMember {
  name: string;
  role: string;
  responsibilities: string[];
  initials: string;
}

const teamMembers: TeamMember[] = [
  {
    name: 'Алимова Елена',
    role: 'Врач ФД',
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

export function Contacts() {
  return (
    <Layout>
      <div className="max-w-4xl mx-auto py-8 sm:py-12">
        <p className="text-sm text-gray-400 mb-8 sm:mb-12">Команда проекта</p>

        <div className="space-y-6">
          {teamMembers.map((member, index) => (
            <div
              key={index}
              className="flex items-start gap-4 sm:gap-5"
            >
              <div className="flex-shrink-0 w-12 h-12 rounded-full bg-gradient-to-br from-rose-400 to-purple-400 flex items-center justify-center text-white text-sm font-semibold">
                {member.initials}
              </div>
              <div className="min-w-0">
                <h3 className="text-base font-semibold text-gray-900">
                  {member.name}
                </h3>
                <p className="text-sm text-rose-500 mb-2">
                  {member.role}
                </p>
                <div className="flex flex-wrap gap-2">
                  {member.responsibilities.map((r, idx) => (
                    <span
                      key={idx}
                      className="inline-block px-2.5 py-0.5 text-xs text-gray-500 bg-gray-100 rounded-full"
                    >
                      {r}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-12 pt-8 border-t border-gray-100 space-y-4">
          <p className="text-sm text-gray-500 leading-relaxed">
            <span style={{ fontFamily: "'Prosto One', cursive" }} className="text-gray-700">Умное сердце</span> — платформа
            для анализа электрокардиограмм с помощью машинного обучения и NLP.
            Объединяем медицинскую экспертизу и разработку ПО для помощи в диагностике
            сердечно-сосудистых заболеваний.
          </p>
          <p className="text-sm text-gray-500">
            Связаться с нами:{' '}
            <a href="mailto:support@smartheart.cloud" className="text-rose-600 hover:text-rose-700 transition-colors">
              support@smartheart.cloud
            </a>
          </p>
        </div>
      </div>
    </Layout>
  );
}
