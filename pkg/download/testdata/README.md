# Commands used to generate certificates for tests
These commands rely on the CSR, key, and config files that should already exist in `pkg/download/testdata/`.

Generate root CA cert:
```
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 3650 -out rootCA.pem
```

Generate server cert:
```
openssl x509 -req \
    -extfile openssl.cnf \
    -extensions v3_req \
    -in server.csr \
    -CA rootCA.pem \
    -CAkey rootCA.key \
    -CAcreateserial \
    -out server.pem \
    -days 3650 -sha256
```

Generate client cert:
```
openssl x509 -req \
    -extfile openssl.cnf \
    -extensions v3_req \
    -in client.csr \
    -CA rootCA.pem \
    -CAkey rootCA.key \
    -CAcreateserial \
    -out client.pem \
    -days 3650 -sha256
```
