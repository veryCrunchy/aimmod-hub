FROM node:22-alpine AS build
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM nginx:1.27-alpine
WORKDIR /usr/share/nginx/html

COPY --from=build /app/dist ./
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/entrypoint.sh /docker-entrypoint.d/40-aimmod-runtime-config.sh

RUN chmod +x /docker-entrypoint.d/40-aimmod-runtime-config.sh

EXPOSE 8080
