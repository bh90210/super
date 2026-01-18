FROM --platform=$BUILDPLATFORM golang:1.25.5 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

WORKDIR /src/server
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -o /out/server .

FROM scratch
WORKDIR /super
COPY --from=builder /out/server ./server
EXPOSE 8888
ENTRYPOINT ["./server"]
