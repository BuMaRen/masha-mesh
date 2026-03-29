#!/bin/bash

set -e

# Ensure non-interactive behavior: remove previous artifacts to avoid overwrite prompt.
rm -f build/certs/tls.key build/certs/tls.csr build/certs/tls.crt

# Use EC key generation to avoid RSA entropy stalls on low-entropy VMs.
openssl ecparam -genkey -name prime256v1 -noout -out build/certs/tls.key

openssl req -new -key build/certs/tls.key -out build/certs/tls.csr -config build/certs/san.conf

openssl x509 -req -in build/certs/tls.csr -signkey build/certs/tls.key -out build/certs/tls.crt -days 365 -extensions v3_req -extfile build/certs/san.conf