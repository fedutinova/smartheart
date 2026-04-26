/**
 * H2 Hypothesis Verification Dataset
 *
 * Synthetic ECG test set for measuring:
 * - Image suitability for automatic analysis
 * - Masked area ratio (baseline vs OCR mode)
 * - Direct_identifier_leak_rate (residual PII after redaction)
 * - P95 image preparation time
 *
 * Dataset structure:
 * - Each sample is a synthetic ECG image with known identifiers in standard locations
 * - Test cases cover: different paper speeds, different patient ID formats, different image qualities
 */

export interface H2TestCase {
  id: string;
  name: string;
  description: string;
  imageBase64: string; // Placeholder - will be populated with actual test images
  expectedIdentifiers: string[]; // Known PII that should be detected and masked
  metadata: {
    paperSpeed: number; // mm/s
    quality: 'good' | 'degraded' | 'low';
    identifierLocations: Array<{ text: string; position: 'top' | 'bottom' | 'left' | 'right' }>;
  };
}

/**
 * TODO: Generate synthetic ECG test cases
 *
 * Requirements:
 * 1. At least 10 test images with varying:
 *    - Paper speeds (25, 50 mm/s)
 *    - Quality levels (good scan, slightly degraded, low contrast)
 *    - Patient ID formats (Cyrillic names, numeric IDs, dates)
 *
 * 2. Each image should:
 *    - Be a valid PNG/JPEG
 *    - Contain ECG waveform (can be synthetic)
 *    - Have identifiable text in known locations
 *    - Have resolution ~1200x800px (standard ECG scan size)
 *
 * 3. Known identifiers to embed:
 *    - Patient name (e.g., "Иван Петров")
 *    - Patient ID (e.g., "12345678")
 *    - Date/time (e.g., "25.04.2026 14:30")
 *    - Doctor name (optional, if present should be masked)
 */
export const H2_TEST_CASES: H2TestCase[] = [
  // Placeholder - to be populated
  {
    id: 'h2_001_good_quality_standard',
    name: 'Good Quality ECG - Standard Format',
    description: 'High-quality scan, Cyrillic patient name at top, ID at bottom-left',
    imageBase64: 'placeholder_base64_string',
    expectedIdentifiers: ['Иван Петров', '12345678', '25.04.2026'],
    metadata: {
      paperSpeed: 25,
      quality: 'good',
      identifierLocations: [
        { text: 'Иван Петров', position: 'top' },
        { text: '12345678', position: 'bottom' },
        { text: '25.04.2026', position: 'bottom' },
      ],
    },
  },
  // Add more test cases following the same pattern
];

/**
 * Utility to load test case and decode base64 to blob
 */
export async function loadTestCase(testCaseId: string): Promise<Blob> {
  const testCase = H2_TEST_CASES.find((tc) => tc.id === testCaseId);
  if (!testCase) {
    throw new Error(`Test case not found: ${testCaseId}`);
  }

  const binary = atob(testCase.imageBase64);
  const array = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    array[i] = binary.charCodeAt(i);
  }

  return new Blob([array], { type: 'image/png' });
}
