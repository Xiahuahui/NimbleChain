# step1: 初始化工作目录
# step2: 启动nfs
# step3：挂载工作目录
# TODO 这需要自己把remote host上的nfs装好

setup:
	bash scripts/setup-cdt.sh

# step1:解绑挂载点
# step2:停掉nfs
# step3:删除工作目录
purge:
	bash scripts/purge-cdt.sh

#发送镜像文件 我们的项目想只用一个镜像，其他的通过配置文件，或者不同的二进制的文件来控制
# step1:拉取所需要的镜像
# step2:为镜像打上相应的tag
# step3:本地 docker save
# step4:将镜像分发给其他容器
# step4:远程 docker load
distribute-docker-images:
	@echo "save docker images to tar file"
	# 将镜像文件拉取下来，并赋予相应的tag，将镜像保存为.tar文件
	bash scripts/save-docker.sh
	# @echo "distribute images to each host"
	ansible-playbook ansible/update-image.yaml

# 配置docker的远程调用的环境
setup-config:
	bash scripts/setup-docker-config.sh

#这个主要是生成配置文件以及docker-compose文件
generate-config:
    # 参数: 节点数,物理节点数
	bash scripts/generate-config.sh 32 1
	bash scripts/generate-cc.sh 32
	bash scripts/generate-docker-compose.sh 32 1

#这个主要是负责将网络启动（每次测试前使用）step1:先将docker-compose.yaml上传到管理机器上；step2:启动网络
deploy-up:
	@echo "deploy & boot tendermint network."
	ansible-playbook ansible/deploy-up.yaml

#这个主要是负责将网络停下来，并且清除相应的配置文件（每次测试完成后使用）
deploy-down:
	@echo "stop tendermint network & clean all docker containers"
	ansible-playbook ansible/deploy-down.yaml
	sudo rm -rf build/config_path/node*
	bash scripts/delete_pcc.sh

#使用的时候要改变一下日志的目录
save-log:
	ansible-playbook ansible/save-log.yaml

#先产生发送事务的脚本,涉及的参数发送时间、发送速率、发送的节点数
start-cdt:
	@echo "Starting use tm-load-test to test tendermint..."
	python scripts/test-generate.py 1 3 200 1000
	bash scripts/start-test.sh
get-height:
	python scripts/get_height.py 1 4 restult.csv

test-ansible:
	ansible all -m ping

# abci 或者 docker-compose.yaml 带宽成本 ，暂停mempool的设定

#先产生发送事务的脚本,涉及的参数发送时间、发送速率、发送的节点数
start-cdt-test:
	@echo "Starting use tm-load-test to test tendermint..."
	python scripts/test-generate.py 1 3 20 100
	# ansible-playbook ansible/start-test.yaml

#测试拥塞控制的脚本代码
test-cc:
	ansible-playbook ansible/start_cc.yaml