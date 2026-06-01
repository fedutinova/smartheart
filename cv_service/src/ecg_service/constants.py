RHYTHM_CLASSES = [
    "SINUS_GROUP", "AFIB", "AFLT", "SVTAC", "VTAC", "VFIB_VFLT", "PACE", "OTHER_UNKNOWN"
]

ACTIVE_RHYTHM_CLASS_NAMES = [
    "SINUS_GROUP", "AFIB", "AFLT", "SVTAC", "VTAC", "VFIB_VFLT", "PACE"
]

RHYTHM_FRIENDLY_RU = {
    "SINUS_GROUP": "синусовый ритм",
    "AFIB": "фибрилляция предсердий",
    "AFLT": "трепетание предсердий",
    "SVTAC": "наджелудочковая тахикардия",
    "VTAC": "желудочковая тахикардия",
    "VFIB_VFLT": "фибрилляция/трепетание желудочков",
    "PACE": "пейсмейкерный ритм",
}

BINARY_TARGETS = ["lvh_label","stt_label","clbbb","crbbb","irbbb","lafb","1avb","imi","asmi"]

BINARY_FRIENDLY_RU = {
    "lvh_label": "возможны признаки гипертрофии левого желудочка",
    "stt_label": "возможны неспецифические изменения ST-T",
    "clbbb": "возможны признаки полной блокады левой ножки пучка Гиса",
    "crbbb": "возможны признаки полной блокады правой ножки пучка Гиса",
    "irbbb": "возможны признаки неполной блокады правой ножки пучка Гиса",
    "lafb": "возможны признаки блокады передней ветви левой ножки",
    "1avb": "возможны признаки AV-блокады I степени",
    "imi": "возможны нижние рубцово-ишемические изменения",
    "asmi": "возможны передне-перегородочные рубцово-ишемические изменения",
}

IMAGENET_MEAN = [0.485, 0.456, 0.406]
IMAGENET_STD  = [0.229, 0.224, 0.225]

PAGE2X2_IMG_SIZE = 384
PAGE2X2_TILE_SIZE = 224
PAGE2X2_OVERLAP_FRAC = 0.04
LEAD_IMG_SIZE = 160

LEAD_NAMES = [
    "I", "II", "III", "aVR", "aVL", "aVF",
    "V1", "V2", "V3", "V4", "V5", "V6",
    "RHYTHM_STRIP"
]
LEAD_TO_IDX = {k: i for i, k in enumerate(LEAD_NAMES)}

LAYOUT_FAMILY_TO_IDX = {
    "3x4_rhythm": 0,
    "3x4": 1,
    "6x2": 2,
    "12x1": 3,
    "unknown": 4,
}
IDX_TO_LAYOUT_FAMILY = {v: k for k, v in LAYOUT_FAMILY_TO_IDX.items()}

DEFAULT_STYLE_REF = {
    "mean": 235.0,
    "std": 18.0,
    "p01": 140.0,
    "p05": 180.0,
    "p25": 225.0,
    "p50": 240.0,
    "p75": 248.0,
    "p95": 252.0,
    "p99": 254.0
}
