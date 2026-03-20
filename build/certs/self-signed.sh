openssl genrsa -out build/certs/test/tls.key 2048

openssl req -new -key build/certs/test/tls.key -out build/certs/test/tls.csr -config build/certs/san.conf

openssl x509 -req -in build/certs/test/tls.csr -signkey build/certs/test/tls.key -out build/certs/test/tls.crt -days 365 -extensions v3_req -extfile build/certs/san.conf