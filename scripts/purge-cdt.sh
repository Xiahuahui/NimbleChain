# echo "1. umount file from remote host"
# ansible-playbook ansible/umount-nfs.yaml

echo "2. stop & rm nfs server."
bash scripts/end-nfs.sh

echo "3. clean work_dir on remote host"
ansible-playbook ansible/clean.yaml

echo "Finish."