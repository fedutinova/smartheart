/// <reference types="node" />

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

interface H2ManifestEntry {
  id: string;
  image_file: string;
  image_path: string;
  full_name: string;
  birth_date: string;
  patient_id: string;
  record_no: string;
  sex: string;
  ecg_date: string;
  expected_identifiers: string[];
  overlay_position: {
    left_x: string;
    left_y: string;
    right_x: string;
    right_y: string;
    font_size: string;
  };
}

export interface H2TestCase {
  id: string;
  name: string;
  description: string;
  imagePath: string;
  expectedIdentifiers: string[];
  metadata: {
    fullName: string;
    birthDate: string;
    patientId: string;
    recordNo: string;
    sex: string;
    ecgDate: string;
    overlayPosition: H2ManifestEntry['overlay_position'];
    source: 'with-test-data';
  };
}

function resolveRepoPath(relativeToRepoRoot: string): string {
  const fromFrontendDir = resolve(process.cwd(), '..', relativeToRepoRoot);
  if (existsSync(fromFrontendDir)) {
    return fromFrontendDir;
  }

  return resolve(process.cwd(), relativeToRepoRoot);
}

const manifestPath = resolveRepoPath('h2/with-test-data-manifest.json');

const manifest = JSON.parse(
  readFileSync(manifestPath, 'utf-8'),
) as H2ManifestEntry[];

export const H2_TEST_CASES: H2TestCase[] = manifest.map((entry) => ({
  id: entry.id,
  name: entry.image_file,
  description: `ECG photo with synthetic identifiers (${entry.image_file})`,
  imagePath: entry.image_path,
  expectedIdentifiers: entry.expected_identifiers,
  metadata: {
    fullName: entry.full_name,
    birthDate: entry.birth_date,
    patientId: entry.patient_id,
    recordNo: entry.record_no,
    sex: entry.sex,
    ecgDate: entry.ecg_date,
    overlayPosition: entry.overlay_position,
    source: 'with-test-data',
  },
}));

export async function loadTestCase(id: string): Promise<Blob> {
  const testCase = H2_TEST_CASES.find((candidate) => candidate.id === id);
  if (!testCase) {
    throw new Error(`H2 test case not found: ${id}`);
  }

  const preferredImagePath = resolveRepoPath(testCase.imagePath);
  const fallbackImagePath = resolveRepoPath(`h2/${testCase.name}`);
  const sourcePath = existsSync(preferredImagePath) ? preferredImagePath : fallbackImagePath;
  const imageBuffer = readFileSync(sourcePath);
  return new Blob([imageBuffer], { type: 'image/png' });
}
