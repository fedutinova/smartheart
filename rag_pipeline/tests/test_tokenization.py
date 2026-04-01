from rag_pipeline.tokenization import medical_ru_tokenizer


class TestMedicalRuTokenizer:
    def test_empty(self):
        assert medical_ru_tokenizer("") == []
        assert medical_ru_tokenizer(None) == []

    def test_basic_russian(self):
        tokens = medical_ru_tokenizer("Нормальный синусовый ритм")
        assert tokens == ["нормальный", "синусовый", "ритм"]

    def test_lowercase(self):
        tokens = medical_ru_tokenizer("ЭКГ АНАЛИЗ")
        assert all(t == t.lower() for t in tokens)

    def test_yo_normalization(self):
        tokens = medical_ru_tokenizer("ёлка Ёж")
        assert "елка" in tokens
        assert "еж" in tokens

    def test_compound_with_hyphen(self):
        tokens = medical_ru_tokenizer("IV-образный")
        assert "iv-образный" in tokens

    def test_compound_with_slash(self):
        tokens = medical_ru_tokenizer("ЭКГ/ЭхоКГ")
        assert "экг/эхокг" in tokens

    def test_compound_with_dot(self):
        tokens = medical_ru_tokenizer("стр.42")
        assert "стр.42" in tokens

    def test_numbers(self):
        tokens = medical_ru_tokenizer("QT 420 мс")
        assert "qt" in tokens
        assert "420" in tokens
        assert "мс" in tokens

    def test_mixed_latin_cyrillic(self):
        tokens = medical_ru_tokenizer("P-wave в отведении V1")
        assert "p-wave" in tokens
        assert "в" in tokens
        assert "отведении" in tokens
        assert "v1" in tokens

    def test_punctuation_stripped(self):
        tokens = medical_ru_tokenizer("Заключение: норма.")
        assert "заключение" in tokens
        assert "норма" in tokens
        assert ":" not in "".join(tokens)

    def test_decimal_number(self):
        tokens = medical_ru_tokenizer("1.5 мм")
        assert "1.5" in tokens
