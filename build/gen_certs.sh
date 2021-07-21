#!/bin/bash -e

#NAME="appqos-pod.appqos"
#ALT1="appqos-silpixa00401062.appqos"
#ALT2="appqos-silpixa00397827d.appqos"
#NAME="*.appqos"
NAME="localhost"

#openssl_latest req -nodes -x509 -newkey rsa:4096 -keyout certs/public/ca.key -out certs/public/ca.crt -days 365 -subj "/O=AppQoS/OU=root/CN=${NAME}" -extensions v3_req -addext "subjectAltName = DNS:${ALT1},DNS:${ALT2}"
openssl req -nodes -x509 -newkey rsa:4096 -keyout /etc/certs/public/ca.key -out /etc/certs/public/ca.crt -days 365 -subj "/O=AppQoS/OU=root/CN=${NAME}"
chmod 644 /etc/certs/public/ca.key

#openssl req -nodes -newkey rsa:3072 -keyout certs/public/appqos.key -out certs/public/appqos.csr -subj "/O=AppQoS/OU=AppQoS Server/CN=${NAME}" -extensions v3_req -addext "subjectAltName = DNS:${ALT1},DNS:${ALT2}"
openssl req -nodes -newkey rsa:3072 -keyout /etc/certs/public/appqos.key -out /etc/certs/public/appqos.csr -subj "/O=AppQoS/OU=AppQoS Server/CN=${NAME}"
openssl x509 -req -in /etc/certs/public/appqos.csr -CA /etc/certs/public/ca.crt -CAkey /etc/certs/public/ca.key -CAcreateserial -out /etc/certs/public/appqos.crt
chmod 644 /etc/certs/public/appqos.crt /etc/certs/public/appqos.csr /etc/certs/public/ca.key /etc/certs/public/appqos.key
