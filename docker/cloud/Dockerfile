FROM --platform=$BUILDPLATFORM golang:1.26.11-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-X main.version=${VERSION}" -o /out/qbit-jay ./cmd/qbit-jay

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 engram
USER engram
WORKDIR /home/engram
COPY --from=builder /out/qbit-jay /usr/local/bin/qbit-jay

EXPOSE 18080
ENTRYPOINT ["qbit-jay"]
CMD ["cloud", "serve"]
