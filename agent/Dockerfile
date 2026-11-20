# Discovery agent: standard library, plus Playwright and Chromium for the
# study's real browser (docs/agent.md). EQ_BROWSER=off studies without it.
FROM python:3.13-slim
ARG VERSION=dev
ARG PLAYWRIGHT_VERSION=1.63.0
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 EQ_VERSION=${VERSION} PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN pip install --no-cache-dir "playwright==${PLAYWRIGHT_VERSION}" \
 && playwright install --with-deps --only-shell chromium \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY equalizer_agent /app/equalizer_agent
RUN useradd --system --uid 10001 agent
USER agent
ENTRYPOINT ["python", "-m", "equalizer_agent"]
