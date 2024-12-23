echo "Boot nfs server."

docker run -d --name nfs \
--privileged \
-v ./build:/nfsshare \
-e SHARED_DIRECTORY=/nfsshare \
-p 2049:2049 itsthenetwork/nfs-server-alpine:12
