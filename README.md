# NimbleChain

### Intro
NimbleChain, which leverages a lightweight reinforcement learning technique to dynamically adjust the timeout threshold
at each node in a distributed fashion.

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

> This document is based on the Ansible tool and is used to deploy and test the tendermint system in a master-slave mode. It is a tool for deploying and testing the blockchain tendermint system, with the following features:
>
> - Simplified deployment.
> - Supports multi-node deployment of blockchain network using Ansible, including distributing Docker images, modifying Docker daemon configurations, and starting and stopping the tendermint network.
> - Template-based configuration file to simplify the process of generating configuration files for multiple nodes.
> - Currently supports Tendermint.
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

  Installation is complete. Next, you can prepare to run it.

# Quick Start

> Use default settings to start quickly. In multi-node operation mode (all hosts meet the runtime requirements), run the DT Docker container on a single host, and run the Tendermint network on other remote hosts.

- Environment configuration.

  | host_name | IP            | role  |
    |-----------|---------------|-------|
  | host_A    | 10.46.173.108 | DT    |
  | host_B    | 10.77.110.163 | node0 |
  | host_C    | 10.77.110.164 | node1 |


1. SSH passwordless login.

Add the SSH public key of host A to hosts B and C, and configure passwordless SSH login for hosts B and C.

2. Ansible host configuration.

   ```shell


   vim /etc/ansible/hosts

   [dt]
   10.77.110.163 ip=10.77.110.163
   10.77.110.164 ip=10.77.110.164

   # test connectivity.
   ansible dt -m ping
   ```
3. Configure the environment for hosts A, B, and C.

   All the following operations are to be executed on host A, in the working directory NimbleChain.

   ```shell
   # set work environment 
   make setup-dt

   # download tendermint image and distribute them on host B、C 
   make distribute-docker-images

   # setup work environment nodes host B、C
   # update docker config [use reomte docker stat api]
   make setup-config
   ```
- Start the Tendermint network for testing.

    - Modify the DT configuration config.yaml.

      Please refer to the 【config.yaml】 Explanation for information about the config.yaml file.
    - Generate Tendermint node configuration.

      ```shell
      # operations:
      # 1. generate tendermint config
      # 2. generate docker-compose files 
      make generate-config
      ```
    - Start the Tendermint network.

      ```shell
      # use ansible to boot tendermint network on host B、C
      make deploy-up
      ```
    - Pull the experimental results from the server.
      ```shell
      make start-log
      ```
- Clean up the environment after the test is completed.

    1. Shut down the Tendermint network.

       ```shell
       # use ansible to close tendermint docker containers on host B、C
       # stop&rm tendermint's docker container on each host
       make deploy-down
       ```
    2. [Optional]Reset the environment.

  ```shell
  # 1. rm tendermint docker image.
  # 2. rm -rf workspace on each host
  # 3. (non-implements) restore docker config on each host
  make purge
  ```
  
# Explanation of config.yaml.

```yaml
version: '3'

config:
  tag: tendermint:test #Version of the image version.
  core_num: 8     #  Number of Docker cores in the experiment.
  block_size: 10000     #  Block size in the experiment.
  binary_tag: propose_timeout  #Version of Tendermint being tested.
  app: kvstore     #Application type on the ABCI end.
  propose_timeout: 5 #Initial propose_timeout set on each node. 5s
  service: fixed     # Method to dynamically adjust timeout. [fixed、FireLedger、NimbleChain-J、NimbleChain-NA、NimbleChain-Full]
  ByzantineNodeList: []  #List of Byzantine nodes.
  CrashNodeList: [0, 3, 6, 9, 12, 15, 18, 22, 26, 29]     # List of Crash nodes.
  reward:
    version: "v3"    #Version of the reward function.
    para: -2         #Value of parameter "a" in the reward function.
```