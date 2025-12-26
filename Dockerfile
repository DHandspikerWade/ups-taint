FROM golang:1.25-bookworm AS build
WORKDIR /go
COPY . .
RUN chmod +x build.sh && ./build.sh

FROM debian:bookworm-slim

ENV NUT_USERNAME=nut
ENV NUT_PASSWORD=123
ENV NUT_ADDRESS=127.0.0.1

COPY --from=build /go/bin/ups-taint /bin/
ENTRYPOINT [ "/bin/ups-taint" ]
