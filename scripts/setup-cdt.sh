# echo "pull nfs image from docker dub"
# docker pull itsthenetwork/nfs-server-alpine:12


echo "1. init work_dir on remote host"
ansible-playbook ansible/init.yaml

# echo "2. start nfs server on port 53"
# bash scripts/start-nfs.sh

# echo "3. mount file to share"
# ansible-playbook ansible/mount-nfs.yaml

# echo "Finish."
