import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';
import { Analyze } from './Analyze';

const { mockAddJob, mockSetError, mockSubmitAnalysis, mockSubmitAnalysisFile } = vi.hoisted(() => ({
  mockAddJob: vi.fn(),
  mockSetError: vi.fn(),
  mockSubmitAnalysis: vi.fn(),
  mockSubmitAnalysisFile: vi.fn(),
}));

const mockImageState = {
  step: 'select' as 'select' | 'crop' | 'ready',
  previewSrc: null as string | null,
  croppedBlob: null as Blob | null,
  croppedPreview: null as string | null,
  error: '',
  handleFileSelect: vi.fn(),
  handleCropComplete: vi.fn(),
  handleCropCancel: vi.fn(),
  rotateImage: vi.fn(),
  handleRecrop: vi.fn(),
  reset: vi.fn(),
  setError: mockSetError,
};

vi.mock('@/services/api', () => ({
  ecgAPI: {
    submitAnalysis: mockSubmitAnalysis,
    submitAnalysisFile: mockSubmitAnalysisFile,
  },
}));

vi.mock('@/components/Layout', () => ({
  Layout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock('@/components/ImageCropper', () => ({
  ImageCropper: () => <div>ImageCropper</div>,
}));

vi.mock('@/components/PaymentModal', () => ({
  PaymentModal: () => <div>PaymentModal</div>,
}));

vi.mock('@/components/CalibrationForm', () => ({
  CalibrationForm: () => <div>CalibrationForm</div>,
}));

vi.mock('@/hooks/usePendingJobs', () => ({
  usePendingJobs: () => ({
    addJob: mockAddJob,
  }),
}));

vi.mock('@/hooks/useQuota', () => ({
  useQuota: () => ({
    quota: {
      needs_payment: false,
      used_today: 0,
      free_remaining: 3,
      daily_limit: 3,
      paid_analyses_remaining: 0,
    },
    refetch: vi.fn(),
  }),
}));

vi.mock('@/hooks/useImageInput', () => ({
  useImageInput: () => mockImageState,
}));

function renderAnalyze() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/analyze']}>
        <Routes>
          <Route path="/analyze" element={<Analyze />} />
          <Route path="/dashboard" element={<div>Главная</div>} />
          <Route path="/results/:id" element={<div>Результат анализа</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('Analyze', () => {
  beforeEach(() => {
    mockAddJob.mockReset();
    mockSetError.mockReset();
    mockSubmitAnalysis.mockReset();
    mockSubmitAnalysisFile.mockReset();

    mockImageState.step = 'select';
    mockImageState.previewSrc = null;
    mockImageState.croppedBlob = null;
    mockImageState.croppedPreview = null;
    mockImageState.error = '';
    sessionStorage.clear();
  });

  it('shows an error when URL mode is submitted without a link', async () => {
    const user = userEvent.setup();
    renderAnalyze();

    await user.click(screen.getByRole('button', { name: 'Вставить ссылку на изображение' }));
    await user.click(screen.getByRole('checkbox'));

    fireEvent.submit(screen.getByRole('button', { name: 'Запустить анализ' }).closest('form')!);

    expect(mockSetError).toHaveBeenCalledWith('Введите URL изображения');
    expect(mockSubmitAnalysis).not.toHaveBeenCalled();
  });

  it('shows an error when file mode is submitted without a prepared image', async () => {
    const user = userEvent.setup();
    mockImageState.step = 'ready';

    renderAnalyze();

    await user.click(screen.getByRole('checkbox'));

    fireEvent.submit(screen.getByRole('button', { name: 'Запустить анализ' }).closest('form')!);

    expect(mockSetError).toHaveBeenCalledWith('Выберите и обрежьте изображение');
    expect(mockSubmitAnalysisFile).not.toHaveBeenCalled();
  });

  it('submits URL analysis and redirects to the result page', async () => {
    const user = userEvent.setup();
    mockSubmitAnalysis.mockResolvedValue({
      request_id: 'req-123',
      job_id: 'job-123',
      status: 'pending',
      message: 'queued',
    });

    renderAnalyze();

    await user.click(screen.getByRole('button', { name: 'Вставить ссылку на изображение' }));
    await user.type(screen.getByPlaceholderText('https://example.com/ekg.jpg'), 'https://example.com/ekg.jpg');
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: 'Запустить анализ' }));

    await waitFor(() => {
      expect(mockSubmitAnalysis).toHaveBeenCalledWith({
        image_temp_url: 'https://example.com/ekg.jpg',
        age: undefined,
        sex: undefined,
        paper_speed_mms: 25,
        mm_per_mv_limb: 10,
        mm_per_mv_chest: 10,
      });
    });
    expect(mockAddJob).toHaveBeenCalledWith('req-123');
    expect(await screen.findByText('Результат анализа')).toBeInTheDocument();
  });
});
