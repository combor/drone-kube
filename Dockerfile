FROM golang:1.10 as builder
RUN go get -d -v github.com/combor/drone-kube
WORKDIR /go/src/github.com/combor/drone-kube
RUN CGO_ENABLED=0 GOOS=linux go build -o drone-kube .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates

COPY --from=builder /go/src/github.com/combor/drone-kube/drone-kube /bin/drone-kube
ENTRYPOINT ["/bin/drone-kube"]
