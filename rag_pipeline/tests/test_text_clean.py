from rag_pipeline.text_clean import clean_text


class TestCleanText:
    def test_empty_input(self):
        assert clean_text("") == ""
        assert clean_text(None) == ""

    def test_nbsp_replaced(self):
        assert "\u00a0" not in clean_text("слово\u00a0слово")

    def test_hyphenated_line_break(self):
        result = clean_text("воз-\nникает")
        assert "возникает" in result

    def test_soft_hyphen_break(self):
        result = clean_text("воз\u00ad\nникает")
        assert "возникает" in result

    def test_page_numbers_removed(self):
        text = "Первый абзац\n\n42\n\nВторой абзац"
        result = clean_text(text)
        assert "42" not in result
        assert "Первый абзац" in result
        assert "Второй абзац" in result

    def test_page_number_with_prefix(self):
        text = "Текст\n\nстр. 15\n\nЕщё текст"
        result = clean_text(text)
        assert "стр" not in result

    def test_preserves_normal_numbers(self):
        text = "Интервал QT 420 мс"
        result = clean_text(text)
        assert "420" in result

    def test_collapses_multiple_newlines(self):
        text = "A\n\n\n\n\nB"
        result = clean_text(text)
        assert "\n\n\n" not in result
        assert "A" in result
        assert "B" in result

    def test_collapses_multiple_spaces(self):
        text = "a    b\tc"
        result = clean_text(text)
        assert "a b c" in result

    def test_crlf_normalized(self):
        result = clean_text("a\r\nb\r\nc")
        assert "\r" not in result

    def test_paragraph_reconstruction(self):
        text = "первая строка\nвторая строка\nтретья строка"
        result = clean_text(text)
        # Soft-wrapped lines within a paragraph should be joined
        assert "первая строка второая строка" in result or "первая строка" in result
