# H2 Набор Изображений ЭКГ

Набор данных для верификации гипотезы H2: сравнение полосного и точечного обезличивания ЭКГ.

## Воспроизведение результатов

Изображения не хранятся в репозитории (720 МБ). Для воспроизведения:

1. Скачать набор с Zenodo:
   **https://zenodo.org/records/20329939**

2. Распаковать и положить содержимое папки `with-test-data/` в `h2/with-test-data/`

3. Запустить верификацию:
   ```bash
   python3 scripts/verify_h2_ground_truth.py
   ```

Результаты появятся в:
- `docs/artifacts/h2-ground-truth-validation.md` — сводный отчёт
- `docs/artifacts/source-data/redaction/h2_ground_truth_results.csv` — детальные данные по каждому изображению

Готовые результаты уже зафиксированы в репозитории и доступны без запуска скрипта.

## Состав набора

- 100 фотографий ЭКГ из датасета [GenECG (Dataset B)](https://huggingface.co/datasets/edcci/GenECG/tree/main/Dataset_B_ECGs_with_imperfections)
- На каждое изображение вручную наложены синтетические персональные данные (ФИО, дата рождения, ID пациента, номер записи)
- Координаты и содержимое наложений описаны в `with-test-data-manifest.json`

## Файлы в репозитории

| Файл | Описание |
|---|---|
| `with-test-data-manifest.json` | Манифест набора: идентификаторы и координаты наложения |
| `with-test-data-manifest.csv` | То же в формате CSV |
| `test-identifiers-template.csv` | Шаблон для добавления новых тестовых случаев |
