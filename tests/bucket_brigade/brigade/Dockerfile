FROM alpine:latest

RUN apk add docker

WORKDIR /koinos
ADD . .

CMD ["cd /koinos && docker-compose up"]
