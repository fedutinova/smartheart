import re

def _join_soft_wrapped_lines(text: str) -> str:
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    # Склейка переносов по дефису: воз-\nникает -> возникает
    text = re.sub(r"([A-Za-zА-Яа-яЁё])-\n([A-Za-zА-Яа-яЁё])", r"\1\2", text)
    # Мягкий перенос
    text = text.replace("\u00ad\n", "")
    parts = re.split(r"\n\s*\n", text)
    norm_parts = []
    for p in parts:
        lines = [ln.strip() for ln in p.split("\n")]
        lines = [ln for ln in lines if ln]
        if not lines:
            continue
        norm_parts.append(" ".join(lines))
    return "\n\n".join(norm_parts)

def clean_text(text: str) -> str:
    if not text:
        return ""
    text = text.replace("\u00a0", " ")  # NBSP
    text = _join_soft_wrapped_lines(text)
    # Базовая нормализация
    text = re.sub(r"[ \t]+", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    # Удалить служебные строки наподобие номеров страниц
    cleaned_pars = []
    for par in text.split("\n\n"):
        p = par.strip()
        if not p:
            continue
        if re.fullmatch(r"(стр\.?\s*)?\d{1,4}", p, flags=re.IGNORECASE):
            continue
        cleaned_pars.append(p)
    return "\n\n".join(cleaned_pars).strip()
