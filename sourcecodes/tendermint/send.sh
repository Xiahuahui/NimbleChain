rm -rf build/tendermint
make build-linux
scp -i ~/.ssh/ruc_500 -r build/tendermint xiahuahui@10.46.173.108:/home/xiahuahui/tendermint_deploy_tool/build/binary/propose_timeout
