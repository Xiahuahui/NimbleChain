# sudo rm -rf /home/xhh/tendemrint_env/build/service/pcc
#!/bin/bash

# 远程服务器的IP地址、用户名和密码
HOST="10.77.110.163"
USER="xhh"

PASSWORD="123456xhh"

# 要删除的文件路径
FILE_PATH="/home/xhh/tendemrint_env/build/service/pcc"

# 使用sshpass命令来提供密码并执行远程删除文件操作
sshpass -p $PASSWORD ssh $USER@$HOST "sudo rm -rf $FILE_PATH"