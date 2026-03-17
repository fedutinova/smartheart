FROM python:3.13-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential git curl && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Layer 1: CPU-only PyTorch — cached independently of requirements.txt.
# Re-downloads only when the base image changes.
RUN pip install --no-cache-dir \
    torch \
    --index-url https://download.pytorch.org/whl/cpu

# Layer 2: everything else — re-runs only when requirements.txt changes.
COPY requirements.txt .
RUN pip install --no-cache-dir \
    --extra-index-url https://pypi.org/simple \
    -r requirements.txt

COPY . .

EXPOSE 8000

CMD ["uvicorn", "api.main:app", "--host", "0.0.0.0", "--port", "8000"]
