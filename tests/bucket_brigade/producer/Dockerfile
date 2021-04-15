FROM alpine:latest

RUN apk add docker

ARG PEER
ENV PEER=${PEER}

WORKDIR /koinos
ADD . .

CMD ["cd /koinos && docker-compose up"]
