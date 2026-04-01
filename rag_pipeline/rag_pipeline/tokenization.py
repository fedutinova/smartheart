import re
import pymorphy3

_TOKEN_RE = re.compile(
    r"[0-9A-Za-zА-Яа-яЁё]+(?:[./-][0-9A-Za-zА-Яа-яЁё]+)*",
    flags=re.UNICODE,
)

_morph = pymorphy3.MorphAnalyzer()

_CYRILLIC_RE = re.compile(r"[А-Яа-яЁё]")


def _lemmatize(token: str) -> str:
    if not _CYRILLIC_RE.search(token):
        return token
    parsed = _morph.parse(token)
    return parsed[0].normal_form if parsed else token


def _normalize_yo(s: str) -> str:
    return s.replace("ё", "е").replace("Ё", "Е")


def medical_ru_tokenizer(text: str) -> list[str]:
    if not text:
        return []
    text = _normalize_yo(text)
    tokens = [t.lower() for t in _TOKEN_RE.findall(text)]
    return [_normalize_yo(_lemmatize(t)) for t in tokens]
