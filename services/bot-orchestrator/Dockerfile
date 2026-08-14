FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY contracts ./contracts
COPY services/bot-orchestrator ./services/bot-orchestrator

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/service ./services/bot-orchestrator

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 appuser
USER appuser

COPY --from=build /out/service /service

ENTRYPOINT ["/service"]
