# NimbleChain

### Intro

### Directories

> - /sourcecodes: source codes of different blockchain systems and agent-side
> - /build: blockchain system generates executable binary files and configuration files
> - /ansible: these playbook.yaml file is used for deploying systems with Ansible
> - /docker-compose: docker-compose files are used to deploy blockchain systems
> - /templates: template files for generating configuration files and docker-compose files
> - ansible.cfg and hosts: is used to manage different servers

### Runtime（all node）

- OS - Ubuntu 20.04 ^
- OS User: root
- python3 & pip3
- docker-compose
- docker API version: 1.12^

### Deploy-Tool

> 本文档是基于ansible工具，用来部署测试tendermint系统，是一个主从模式, 一个部署和测试区块链tendermint工具，具有以下特性：
>
> - 部署简化
> - 支持使用ansible多节点部署区块链网络，包括docker镜像分发，docker daemon配置修改，启动和停止tendermint网络，nfs共享，冗余文件清理
> - 配置文件模板化，简化多节点配置文件生成过程
> - 当前支持tendermint
> - https://zhuanlan.zhihu.com/p/606174368?utm_id=0
> - https://blog.csdn.net/make_progress/article/details/124295978

### Install（master node）

- ansible

  ```shell
  pip3 install ansible
  ```
- jinja2

  ```shell
  pip3 install jinja2
  ```

  安装完成。接下来可以准备运行了。

# Quick Start

> 使用默认设置快速开始。多机运行模式（所有主机符合Runtime要求），使用单台主机运行DT docker容器，其他远程主机运行tendermint网络。

- 环境配置

  | 主机名 | IP            | 角色  |
    | ------ |---------------| ---- |
  | 主机A  | 10.46.173.108 | DT   |
  | 主机B  | 10.77.110.163 | node0 |
  | 主机C  | 10.77.110.164 | node1 |


1. ssh免密登录

将主机A的ssh公钥添加到B、C主机上，并配置B、C主机的ssh免密登录。

2. ansible主机设置

   ```shell


   vim /etc/ansible/hosts

   # 输入以下内容
   [dt]
   10.77.110.163 ip=10.77.110.163
   10.77.110.164 ip=10.77.110.164

   # 测试连通性
   ansible dt -m ping
   ```
3. 配置主机A、B、C的环境

   以下所有操作都在主机A上执行, 工作目录为NimbleChain：

   ```shell
   # set work environment 设置工作环境   
   make setup-dt

   # download tendermint image and distribute them on host B、C [镜像分发]
   make distribute-docker-images

   # setup work environment nodes host B、C
   # !!! 这个操作会关闭目标主机B、C的docker服务，并开放docker服务的远程访问权限、替换daemon.json和docker service
   # update docker config [use reomte docker stat api]
   make setup-config
   ```
- 启动 tendermint 网络进行测试

    - 修改DT配置config.yaml

      有关config.yaml文件的说明请查看#【config.yaml说明】
    - 生成tendermint节点配置

      ```shell
      # operations:
      # 1. generate tendermint config
      # 2. generate docker-compose files 
      make generate-config
      ```
    - 启动tendemint网络

      ```shell
      # use ansible to boot tendermint network on host B、C
      make deploy-up
      ```
    - 从服务器上拉取单次的实验结果
      ```shell
      make start-log
      ```
- 测试完成后【环境清理】

    1. 关闭tendermint网络

       ```shell
       # use ansible to close tendermint docker containers on host B、C
       # stop&rm tendermint's docker container on each host
       make deploy-down
       ```
    2. [可选]重置环境

  ```shell
  # 1. rm tendermint docker image.
  # 2. rm -rf workspace on each host
  # 3. (non-implements) restore docker config on each host
  make purge
  ```

# 运行结果

DT运行结果存放在result文件夹，文件格式为csv。

# config.yaml说明

```yaml
version: '3'

config:
  tag: tendermint:test #镜像的版本的版本
  core_num: 8     #  实验中的docker核数
  block_size: 10000     #  实验中的区块大小
  binary_tag: propose_timeout  #测试的tendermint的版本
  app: kvstore     #abci端的应用类型
  propose_timeout: 5 #每个节点上设置的初始化的propose_timeout 5s
  service: fixed     #动态调整timeout的方式 [fixed、FireLedger、NimbleChain-J、NimbleChain-NA、NimbleChain-Full]
  ByzantineNodeList: []  #拜占庭节点列表
  CrashNodeList: [0, 3, 6, 9, 12, 15, 18, 22, 26, 29]     # 宕机节点列表
  reward:
    version: "v3"    #奖励函数的版本
    para: -2         #奖励函数中a的设置
```