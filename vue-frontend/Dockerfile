# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm config set registry https://registry.npmmirror.com \
 && npm config set fetch-retries 5 \
 && npm config set fetch-retry-mintimeout 20000 \
 && npm config set fetch-retry-maxtimeout 120000 \
 && npm config set fetch-timeout 300000
RUN --mount=type=cache,target=/root/.npm \
  npm ci --cache /root/.npm --prefer-offline --no-audit --no-fund

COPY . .
RUN npm run build

FROM nginx:1.29-alpine

ENV API_UPSTREAM=aidemoapi:9090

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
