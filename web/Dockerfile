FROM node:22-alpine AS build
WORKDIR /app

RUN corepack enable

COPY package.json ./
RUN pnpm install --no-frozen-lockfile

COPY . .
RUN pnpm build:web

FROM nginx:1.27-alpine
WORKDIR /usr/share/nginx/html

COPY --from=build /app/dist ./
COPY web/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/docker/entrypoint.sh /docker-entrypoint.d/40-aimmod-runtime-config.sh

RUN chmod +x /docker-entrypoint.d/40-aimmod-runtime-config.sh

EXPOSE 8080
