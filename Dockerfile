FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pgoctl ./cmd/pgoctl

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /pgoctl /pgoctl

ENTRYPOINT ["/pgoctl"]
