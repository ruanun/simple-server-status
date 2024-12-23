FROM --platform=$BUILDPLATFORM alpine

ARG TZ="Asia/Shanghai"

ENV TZ ${TZ}

RUN apk upgrade \
    && apk add bash tzdata \
    && ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone

WORKDIR /app

COPY sssd sssd

ENV CONFIG="sss-dashboard.yaml"

CMD ["/app/sssd"]