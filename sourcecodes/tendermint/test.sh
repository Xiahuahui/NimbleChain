rm -rf /Users/xiahuahui/.tendermint/
rm -rf propose_time.csv
rm -rf latency_time.csv
rm -rf commit_round.csv
rm -rf cost_time.csv
make build
make install
tendermint init
tendermint node --proxy_app=kvstore
