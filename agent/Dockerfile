# Discovery agent: standard library only, no packages to install.
FROM python:3.13-slim
ARG VERSION=dev
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 EQ_VERSION=${VERSION}
WORKDIR /app
COPY equalizer_agent /app/equalizer_agent
RUN useradd --system --uid 10001 agent
USER agent
ENTRYPOINT ["python", "-m", "equalizer_agent"]
