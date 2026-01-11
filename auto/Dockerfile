FROM --platform=$BUILDPLATFORM golang:1.25.5 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /auto
COPY ./ ./
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o ./auto


FROM scratch
WORKDIR /autp
COPY --from=builder /auto/auto ./auto
EXPOSE 3000
ENTRYPOINT ["./auto"]
