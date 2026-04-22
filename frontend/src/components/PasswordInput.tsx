import { useState } from 'react';

interface PasswordInputProps {
  id: string;
  name: string;
  placeholder: string;
  value: string;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onInvalid?: (e: React.InvalidEvent<HTMLInputElement>) => void;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  required?: boolean;
  title?: string;
  autoComplete?: string;
}

/**
 * Password input field with a show/hide toggle.
 * Manages visibility state internally and provides consistent eye-toggle icons.
 */
export function PasswordInput({
  id,
  name,
  placeholder,
  value,
  onChange,
  onInvalid,
  minLength,
  maxLength,
  pattern,
  required = true,
  title,
  autoComplete,
}: PasswordInputProps) {
  const [showPassword, setShowPassword] = useState(false);

  const handleInvalid = (e: React.InvalidEvent<HTMLInputElement>) => {
    const input = e.target as HTMLInputElement;
    input.setCustomValidity('');

    if (onInvalid) {
      onInvalid(e);
    } else {
      // Default validation messages
      if (input.validity.valueMissing) {
        input.setCustomValidity('Введите пароль');
      } else if (input.validity.tooShort) {
        input.setCustomValidity(`Пароль должен быть не менее ${minLength} символов`);
      } else if (input.validity.patternMismatch) {
        input.setCustomValidity(title || 'Некорректный формат');
      }
    }
  };

  const handleInput = (e: React.FormEvent<HTMLInputElement>) => {
    (e.target as HTMLInputElement).setCustomValidity('');
  };

  return (
    <div className="relative">
      <input
        id={id}
        name={name}
        type={showPassword ? 'text' : 'password'}
        required={required}
        minLength={minLength}
        maxLength={maxLength}
        pattern={pattern}
        title={title}
        autoComplete={autoComplete}
        className="appearance-none relative block w-full px-4 py-3 pr-11 border border-gray-300 placeholder-gray-400 text-gray-900 rounded-xl focus:outline-none focus:ring-2 focus:ring-rose-500 focus:border-rose-500 sm:text-sm"
        placeholder={placeholder}
        value={value}
        onChange={onChange}
        onInvalid={handleInvalid}
        onInput={handleInput}
      />
      <button
        type="button"
        tabIndex={-1}
        className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600"
        onClick={() => setShowPassword((v) => !v)}
        aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
      >
        {showPassword ? (
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
            <circle cx="12" cy="12" r="3" />
          </svg>
        ) : (
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
            <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
            <line x1="1" y1="1" x2="23" y2="23" />
          </svg>
        )}
      </button>
    </div>
  );
}
