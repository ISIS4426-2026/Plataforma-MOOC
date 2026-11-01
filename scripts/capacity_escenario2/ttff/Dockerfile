# Medidor del tiempo al primer cuadro (G4, #224): Chromium sin ventana + hls.js, como exige A6 §6.
# Debian trae un Chromium con los codecs H.264/AAC (el que baja Puppeteer o Playwright no los trae).
FROM node:20-bookworm-slim
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends chromium fonts-liberation ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY package.json ./
RUN npm install --omit=dev --no-audit --no-fund
COPY ttff.js ./
ENV PUPPETEER_SKIP_DOWNLOAD=1
ENTRYPOINT ["node", "/app/ttff.js"]
