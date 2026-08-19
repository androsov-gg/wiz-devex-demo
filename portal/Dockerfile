# ------------------------------------------------------------------------------
# Stage 1: Build React (Vite) Frontend
# ------------------------------------------------------------------------------
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json ./
RUN npm install

COPY frontend/ ./
RUN npm run build

# ------------------------------------------------------------------------------
# Stage 2: Build Go Backend
# ------------------------------------------------------------------------------
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/backend

COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true

COPY backend/ ./
# Copy compiled frontend dist into backend embed directory
COPY --from=frontend-builder /app/frontend/dist ./dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# ------------------------------------------------------------------------------
# Stage 3: Minimal Production Image (Alpine)
# ------------------------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=backend-builder /app/server ./server

ENV PORT=3000
ENV GIN_MODE=release

EXPOSE 3000
ENTRYPOINT ["/app/server"]
