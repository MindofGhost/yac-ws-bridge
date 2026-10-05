# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /src/bridge-cloud/build
COPY bridge-cloud/build/package*.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY bridge-cloud/index.js bridge-cloud/package.json ../
COPY bridge-cloud/build/build.cjs bridge-cloud/build/verify.cjs ./
ENV OUT_DIR=/out/cloud
RUN npm run build && node --check /out/cloud/index.js && node verify.cjs ../index.js /out/cloud/index.js
FROM scratch AS artifacts
COPY --from=build /out/ /
