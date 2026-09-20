FROM golang:1.26.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dinner-please-bot ./cmd/bot

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/dinner-please-bot /app/dinner-please-bot
COPY config /app/config
COPY recipes /app/recipes
ENTRYPOINT ["/app/dinner-please-bot"]
