# Пакет H2 По Обезличиванию

Эта директория содержит пакет H2 по клиентскому обезличиванию ЭКГ.

## Состав

- [h2_ground_truth_results.csv](/home/anna/Documents/smartheart/docs/artifacts/source-data/redaction/h2_ground_truth_results.csv) - сводные результаты по эталонной разметке;
- [h2_detect_pii_runtime_results.json](/home/anna/Documents/smartheart/docs/artifacts/source-data/redaction/h2_detect_pii_runtime_results.json) - замеры времени оптимизированного `detectPII`;
- [h2_tesseract_runtime_results.json](/home/anna/Documents/smartheart/docs/artifacts/source-data/redaction/h2_tesseract_runtime_results.json) - справочные замеры OCR-контура.

H2 трактуется как подтверждённая на воспроизводимом наборе с эталонной разметкой по критериям геометрии маскирования, сохранности ЭКГ-содержимого, утечек и среднего времени оптимизированного `detectPII`.
