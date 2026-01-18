FROM --platform=$BUILDPLATFORM golang:1.25.5 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .

RUN test -f go.mod || go mod init github.com/bh90210/super/auto
RUN grep -qxF 'replace github.com/go-playground/webhooks/v6 => github.com/bh90210/webhooks/v6 v6.5.2' go.mod || \
printf '\nreplace github.com/go-playground/webhooks/v6 => github.com/bh90210/webhooks/v6 v6.5.2\n' >> go.mod
RUN go mod tidy

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -o /out/auto .

FROM scratch
WORKDIR /auto
COPY --from=builder /out/auto ./auto
EXPOSE 3000
EXPOSE 2112
ENTRYPOINT ["./auto"]