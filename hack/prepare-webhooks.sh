#!/usr/bin/env bash

if [[ -z "$1"  ]]; then
    echo "need a argument (local|cluster)"
    exit 1
fi

case "$1" in
local)
    CERTS="/tmp/k8s-webhook-server/serving-certs/"
    mkdir -p ${CERTS}

    # 创建一个根证书，用它来对服务签署证书
    openssl req \
        -newkey rsa:4096 -nodes -sha256 \
        -x509 -days 3650 \
        -subj "/C=CN/ST=Shanghai/L=Shanghai/O=devops/OU=devops/CN=kubernetes/" \
        -keyout ${CERTS}/root.key \
        -out ${CERTS}/root.crt 


    # 针对 tls 创建证书和私有 key
    openssl req \
        -newkey rsa:4096 -nodes -sha256 \
        -keyout ${CERTS}/tls.key \
        -out ${CERTS}/tls.csr \
        -subj "/C=CN/ST=Shanghai/L=Shanghai/O=devops/OU=devops/CN=tls/"
    openssl x509 \
        -req -days 365 -set_serial 0 -sha256 \
        -CA ${CERTS}/root.crt \
        -CAkey ${CERTS}/root.key \
        -in ${CERTS}/tls.csr \
        -out ${CERTS}/tls.crt
    exit 0
;;
cluster)
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
    exit 0
;;
*)
    echo "Not support argument $1"
    exit 1
esac
