# syntax=docker/dockerfile:1
# Frontend production image — multi-stage build (v2.24).
#
# Stage 1 builds the static assets entirely inside Docker, so a fresh
# clone needs no host Node.js: `docker compose up -d --build` just works.
# This replaces the previous "build dist on the host first
# (./scripts/build_frontend_dist.sh), then COPY dist" flow; that script is
# kept for host-side builds but is no longer a prerequisite.
#
# Stage 2 base image pinned by digest for reproducibility. Do NOT switch
# back to the floating `nginx:stable-alpine` tag: it drifts with every
# stable release and a newer Alpine base (3.24+) fails to start on old
# hosts such as CentOS 7 (kernel 3.10 + old libseccomp), which broke
# v0.7.0. This digest is nginx 1.30.3 / Alpine 3.23.5.
#
# Vite 7 requires Node 20.19+ / 22.12+ — pinned to node:22-alpine.
# Optional build args:
#   --build-arg NPM_REGISTRY=https://registry.npmmirror.com  (CN mirror)
#   --build-arg VITE_FRONTEND_COMMIT=<sha>                   (version stamp)

FROM node:22-alpine AS build
WORKDIR /app

ARG NPM_REGISTRY=
ARG VITE_FRONTEND_COMMIT=unknown

COPY package.json package-lock.json ./
# 本地 file: 依赖（xlsx 的离线 tgz）必须先于 npm ci 进镜像，
# 否则 npm 解析 lockfile 时报 ENOENT。
COPY packages ./packages
# Registry override is opt-in via build arg; empty keeps the default source.
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
	&& npm ci

COPY . .
RUN VITE_IS_DOCKER=true VITE_FRONTEND_COMMIT="$VITE_FRONTEND_COMMIT" npm run build

# ---- runtime stage (unchanged base) ----
FROM nginx:1.30.3-alpine@sha256:0d3b80406a13a767339fbe2f41406d6c7da727ab89cf8fae399e81f780f814d1

COPY --from=build /app/dist /usr/share/nginx/html

COPY nginx.conf /etc/nginx/templates/default.conf.template

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV MAX_FILE_SIZE_MB=50

EXPOSE 80

ENTRYPOINT ["/docker-entrypoint.sh"]
