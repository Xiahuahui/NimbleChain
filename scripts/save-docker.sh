echo "Pull image from dockerhub..."
docker load < images/tendermint_and_rl.tar
echo "Pull finished. Save and tag docker images."

docker tag tendermint/base:latest tendermint:test

docker save tendermint:test -o images/tendermint_base_new.tar