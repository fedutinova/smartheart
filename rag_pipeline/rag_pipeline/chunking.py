import re

_HEADING_RE_1 = re.compile(r"^\s*\d+(?:\.\d+)*[).]?\s+\S+")
_HEADING_RE_2 = re.compile(r"^\s*[A-ZА-ЯЁ0-9][A-ZА-ЯЁ0-9\s()№-]+$")
_HEADING_RE_3 = re.compile(
    r"^\s*(глава|раздел|приложение|таблица|рисунок)\b", re.IGNORECASE,
)

def is_heading(paragraph: str) -> bool:
    p = paragraph.strip()
    if len(p) < 4:
        return False
    is_pattern = _HEADING_RE_1.match(p) or _HEADING_RE_3.match(p)
    if len(p) <= 90 and (p.endswith(":") or is_pattern):
        return True
    return bool(len(p) <= 120 and _HEADING_RE_2.match(p))

def split_to_units(text: str) -> list[str]:
    return [p.strip() for p in text.split("\n\n") if p.strip()]

def _tail_units_for_overlap(units: list[str], overlap_chars: int) -> list[str]:
    if overlap_chars <= 0 or not units:
        return []
    tail = []
    total = 0
    for u in reversed(units):
        add = len(u) + (2 if tail else 0)
        if total + add > overlap_chars:
            break
        tail.append(u)
        total += add
    return list(reversed(tail))

def chunk_units_semantic(
    units: list[str],
    target_chars: int,
    min_chars: int,
    max_chars: int,
    overlap_chars: int,
) -> list[str]:
    chunks: list[str] = []
    cur_units: list[str] = []
    cur_len = 0

    def flush():
        nonlocal cur_units, cur_len
        if not cur_units:
            return
        ch = "\n\n".join(cur_units).strip()
        if ch:
            chunks.append(ch)
        cur_units = []
        cur_len = 0

    for u in units:
        u = u.strip()
        if not u:
            continue

        if is_heading(u) and cur_len >= min_chars:
            flush()

        add_len = len(u) + (2 if cur_units else 0)
        if cur_len + add_len <= max_chars:
            cur_units.append(u)
            cur_len += add_len
        else:
            prev_units = cur_units[:]
            flush()
            overlap_units = _tail_units_for_overlap(prev_units, overlap_chars)
            cur_units = overlap_units[:]
            cur_len = len("\n\n".join(cur_units)) if cur_units else 0

            if len(u) > max_chars:
                sentences = re.split(r"(?<=[.!?])\s+", u)
                buf = ""
                for s in sentences:
                    s = s.strip()
                    if not s:
                        continue
                    if not buf:
                        buf = s
                    elif len(buf) + 1 + len(s) <= max_chars:
                        buf += " " + s
                    else:
                        if cur_units:
                            flush()
                        chunks.append(buf)
                        buf = s
                if buf:
                    if cur_units:
                        flush()
                    chunks.append(buf)
                cur_units = []
                cur_len = 0
            elif cur_units:
                cur_units.append(u)
                cur_len = len("\n\n".join(cur_units))
            else:
                cur_units = [u]
                cur_len = len(u)

        if cur_len >= target_chars:
            flush()

    flush()

    merged: list[str] = []
    for ch in chunks:
        if merged and len(ch) < min_chars:
            merged[-1] = (merged[-1] + "\n\n" + ch).strip()
        else:
            merged.append(ch)
    return merged
