from fabric import task
from fabric import Connection
from fabric import ThreadingGroup
import json
import os
import time







@task
def local(ctx):
    print("local")


@task
def remote(ctx):
    with open('./auth.json','r') as af:
        auth=json.load(af)
        print(auth)
    host_ip="222.19.236.142"
    dest_dir="/home/special/user/chengtaining"
    source_dir="deployyt"
    work_path=dest_dir+'/'+source_dir

    # deploy to remote
    servers=[Connection(host=host_ip,user=auth['user'],port=p) for p in [6011,6017]]
    deployG=ThreadingGroup.from_connections(servers)

    cmd="mkdir {}".format(work_path)
    deployG.run(cmd,hide=True)

    for f in os.listdir(source_dir):
        deployG.put("{}/{}".format(source_dir,f),work_path)
    deployG.close()

    # run benchmark
    # for id,p in enumerate([6018,6011,6017,6019]):
    #     with Connection(host=host_ip,user=auth['user'],port=p) as c:
    #         cmd="cd {};pkill server;nohup ./server -id {} -log_dir=./ -log_level=info -algorithm=mhotstuff > ./dev.out 2>&1 &".format(work_path,id+1)
    #         c.run(cmd,hide=True)
    # print("start finished")
    
    # time.sleep(120)

    # # collect log
    # for p in [6017]:
    #     with Connection(host=host_ip,user=auth['user'],port=p) as c:
    #         cmd="cd {};python statistics.py".format(work_path)
    #         c.run(cmd,hide=True)

            


    print("ok")

