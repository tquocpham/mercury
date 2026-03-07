ssh-keygen -t rsa -b 4096 -m PEM -f jwtRS256.key

# Don't add passphrase
openssl rsa -in jwtRS256.key -pubout -outform PEM -out jwtRS256.key.pub

cat jwtRS256.key

cat jwtRS256.key.pub

aws ssm put-parameter \
  --name "/mercury/jwt-public-key" \
  --type "SecureString" \
  --value "$(cat ./jwtRS256.key.pub)" \
  --region us-west-1 \
  --no-cli-pager

aws ssm get-parameter \
  --name "/mercury/jwt-public-key" \
  --with-decryption --region us-west-1 \
  --no-cli-pager

aws ssm put-parameter \
  --name "/mercury/jwt-private-key" \
  --type "SecureString" \
  --value "$(cat ./jwtRS256.key)" \
  --region us-west-1 \
  --no-cli-pager

aws ssm get-parameter \
  --name "/mercury/jwt-private-key" \
  --with-decryption \
  --region us-west-1 \
  --no-cli-pager

go run $GOROOT/src/crypto/tls/generate_cert.go --host localhost
