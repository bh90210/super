FROM --platform=$BUILDPLATFORM golang:1.25.5 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .

RUN go mod init github.com/bh90210/super/server
RUN go mod tidy

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -o /out/server .

FROM scratch
WORKDIR /super
COPY --from=builder /out/server ./server
EXPOSE 8888
ENTRYPOINT ["./server"]
