FROM golang:1.26 AS builder
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /civicbridge ./cmd/civicbridge
RUN CGO_ENABLED=0 go build -o /civicctl ./cmd/civicctl

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
ENV CIVICBRIDGE_STORAGE_DATA_DIR=/tmp/civicbridge-data
COPY --from=builder /civicbridge /app/civicbridge
COPY --from=builder /civicctl /app/civicctl
EXPOSE 49660
ENTRYPOINT ["/app/civicbridge"]
