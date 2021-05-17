# Commands used to genereate certificates for tests
These commands rely on the csr, key, and cnf files that should already exist in download/testdat/

Generate rootCA cert:
```
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 365 -out rootCA.crt
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
    -days 365 -sha256
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
    -days 365 -sha256
```
