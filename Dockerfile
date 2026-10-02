FROM node:22-bookworm AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist/ ./internal/webui/dist/
ENV CGO_ENABLED=0
RUN go build -o /youchu ./cmd/youchu

FROM gcr.io/distroless/static-debian12
COPY --from=build /youchu /youchu
ENTRYPOINT ["/youchu"]
