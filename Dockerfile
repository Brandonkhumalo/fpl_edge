FROM node:22-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ARG VITE_API_URL=/api/v1
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

FROM golang:1.22-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fpl-edge ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend-build /out/fpl-edge /app/fpl-edge
COPY --from=frontend-build /src/frontend/dist /app/public
ENV APP_ENV=production \
    STATIC_DIR=/app/public
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/fpl-edge"]
