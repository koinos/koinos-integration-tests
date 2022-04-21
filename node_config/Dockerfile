FROM alpine:latest
RUN apk update && \
    apk add rsync \
            bash \
            musl \
            libstdc++

ADD . /koinos-config

CMD [ "bash", "/koinos-config/run.sh" ]
