RHYTHM_CLASSES = [
    "SINUS_GROUP", "AFIB", "AFLT", "SVTAC", "VTAC", "VFIB_VFLT", "PACE", "OTHER_UNKNOWN"
]

ACTIVE_RHYTHM_CLASS_NAMES = [
    "SINUS_GROUP", "AFIB", "AFLT", "SVTAC", "VTAC", "VFIB_VFLT", "PACE"
]

# Coarse rhythm super-classes emitted by model.rhythm_super_head. Kept in sync
# with model.RHYTHM_SUPER_CLASSES. Used by refine_rhythm_probs as a hierarchical
# prior over the fine rhythm classes.
RHYTHM_SUPER_CLASSES = ["SINUS", "ATRIAL", "VENTRICULAR", "PACE", "OTHER"]

# Maps each coarse super-class to the fine ACTIVE_RHYTHM_CLASS_NAMES it parents.
# "OTHER" parents OTHER_UNKNOWN, which is not an active class, so it maps to none.
SUPER_TO_ACTIVE = {
    "SINUS": ["SINUS_GROUP"],
    "ATRIAL": ["AFIB", "AFLT", "SVTAC"],
    "VENTRICULAR": ["VTAC", "VFIB_VFLT"],
    "PACE": ["PACE"],
    "OTHER": [],
}

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

# --- Clinical guard rules -------------------------------------------------
# Post-processing rules that suppress clinically impossible binary findings,
# given the predicted rhythm and inter-finding incompatibilities. These restore
# (in spirit) the synthsafe_pair_rules / real_ultrastrict_rules that the delivery
# bundle excluded from runtime (see bundle_info.json: excluded_runtime_logic).

# Rhythm classes whose wide QRS is intrinsic (ventricular origin), so any
# bundle-branch / fascicular-block label is not interpretable.
VENTRICULAR_RHYTHMS = {"VTAC", "VFIB_VFLT"}

# Rule A — rhythm-conditioned suppression: when pred_code is in `rhythms`, the
# binary findings in `suppress` are not clinically interpretable in that context.
RHYTHM_BINARY_GUARDS = [
    {
        "rhythms": VENTRICULAR_RHYTHMS,
        "suppress": {"clbbb", "crbbb", "irbbb", "lafb"},
        "reason": "проводниковая блокада не интерпретируется при желудочковом ритме (широкий QRS по происхождению)",
    },
    {
        "rhythms": {"PACE"},
        "suppress": {"clbbb"},
        "reason": "желудочковая стимуляция имитирует морфологию ЛНПГ — это не истинная блокада",
    },
    {
        "rhythms": {"AFIB", "AFLT", "SVTAC", "VTAC", "VFIB_VFLT"},
        "suppress": {"1avb"},
        "reason": "AV-блокада I степени требует измеримого PR с различимыми P (синусовый контекст)",
    },
]

# Rule B — mutual exclusion: at most one finding per group survives (highest prob).
MUTUALLY_EXCLUSIVE_BINARY = [
    {
        "group": {"clbbb", "crbbb", "irbbb"},
        "reason": "взаимоисключающие паттерны внутрижелудочкового проведения; оставлен наиболее вероятный",
    },
]

# Rule C — dominance: if `dominant` survives, suppress each subordinate finding.
BINARY_DOMINANCE = [
    {
        "dominant": "clbbb",
        "suppress": {"lafb"},
        "reason": "полная ЛНПГ уже включает блокаду передней ветви — отдельная LAFB избыточна",
    },
]

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
