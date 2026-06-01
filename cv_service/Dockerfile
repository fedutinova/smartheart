FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
        libglib2.0-0 libsm6 libxext6 libxrender1 curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir \
        --index-url https://download.pytorch.org/whl/cpu \
        --extra-index-url https://pypi.org/simple \
        torch==2.4.1 torchvision==0.19.1 \
    && pip install --no-cache-dir -r /app/requirements.txt

# Bake the rhythm classifier checkpoint into the image. The URL points to a
# public GitHub Release asset, so no auth is required at build time. To roll
# the model forward, publish a new release and bump MODEL_URL (or pass it via
# --build-arg in CI). Doing this here keeps the production image immutable —
# no host volume or one-off scp needed when deploying.
ARG MODEL_URL="https://github.com/fedutinova/smartheart/releases/download/stage_oldv9_residual_best/stage_oldv9_residual_best.pt"
RUN mkdir -p /app/models && \
    curl -fsSL -o /app/models/stage_oldv9_residual_best.pt "$MODEL_URL" && \
    test -s /app/models/stage_oldv9_residual_best.pt

COPY src /app/src
COPY config /app/config
COPY tests /app/tests

ENV PYTHONPATH=/app/src
ENV ECG_CHECKPOINT_PATH=/app/models/stage_oldv9_residual_best.pt
ENV ECG_STYLE_REF_PATH=/app/config/synth_style_ref.json
ENV ECG_DEFAULT_PREPROCESS=synthmatch

EXPOSE 8000

CMD ["uvicorn", "ecg_service.api_fastapi:app", "--host", "0.0.0.0", "--port", "8000"]
