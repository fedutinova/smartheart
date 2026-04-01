import re

_TOKEN_RE = re.compile(
    r"[0-9A-Za-zА-Яа-яЁё]+(?:[./-][0-9A-Za-zА-Яа-яЁё]+)*",
    flags=re.UNICODE,
)

def medical_ru_tokenizer(text: str) -> list[str]:
    if not text:
        return []
    text = text.replace("ё", "е").replace("Ё", "Е")
    return [t.lower() for t in _TOKEN_RE.findall(text)]
