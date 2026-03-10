FROM node:22-alpine AS build
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/package.json
RUN pnpm install --frozen-lockfile

COPY web ./web
RUN pnpm build:web

FROM nginx:1.27-alpine
WORKDIR /usr/share/nginx/html

COPY --from=build /app/web/dist ./
COPY web/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/docker/entrypoint.sh /docker-entrypoint.d/40-aimmod-runtime-config.sh

RUN chmod +x /docker-entrypoint.d/40-aimmod-runtime-config.sh

EXPOSE 8080
