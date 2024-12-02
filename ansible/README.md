## Ansible的安装

在ubuntu上安装的步骤以及注意事项，包括调试过程中的一些问题。

只需要在控制节点上安装ansible；

管理节点保证sudo权限，找到sudoer文件  user ALL=(ALL)，保证sudo免密。

ansible的安装有两种方式：

* ```
  sudo apt-get install ansible
  ```
* ```
  pip3 install ansible
  ```

密钥共享：将控制节点的公钥发送到，管理节点上去

建议为每个项目，重新构建ansible的配置文件

模版配置文件的地址：

[https://raw.githubusercontent.com/ansible/ansible/stable-2.9/examples/ansible.cfg]()

解决DNS的方法有两种

* 修改host文件
* 用core/dns等

对于配置文件，先检查当前目录，再检查默认的目录

ssh的原理啊 是将client端的公钥放到sever端 客户端是个私钥  单独替换私钥还是有点问题 要加公钥删掉

ansible_user:为登陆用户，become_user 为执行命令的用户

Roles介绍：

配置Nfs参考[https://zhuanlan.zhihu.com/p/466197600]()


参考[https://blog.csdn.net/xiaochong0302/article/details/128580109]()
