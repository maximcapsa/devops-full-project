# Production image for the Next.js storefront (standalone output).
# Build from the frontend/ directory: docker build -t frontend ./frontend
FROM node:20-alpine AS build
WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .
# Baked at build time: empty => the browser calls the API with relative /v1/*
# URLs, served same-origin through the Traefik ingress (no CORS).
ENV NEXT_PUBLIC_BFF_URL=""
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static

USER node
EXPOSE 3000
CMD ["node", "server.js"]
