openssl genrsa -out build/certs/tls.key 2048

openssl req -new -key build/certs/tls.key -out build/certs/tls.csr -config build/certs/san.conf

openssl x509 -req -in build/certs/tls.csr -signkey build/certs/tls.key -out build/certs/tls.crt -days 365 -extensions v3_req -extfile build/certs/san.conf