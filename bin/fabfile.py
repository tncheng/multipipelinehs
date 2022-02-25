from platform import node
import subprocess
from multiprocessing.dummy import Process
from fabric import task
from fabric import Connection
from fabric import ThreadingGroup
import utils
import os
import time






@task
def local(ctx):
    print("local")


def run(host,user,port,id,work_dir,start,timeout):
    conn=Connection(host=host,user=user,port=port)
    deadline=150
    pid=0
    with conn.cd(work_dir):
        try:
            conn.run('pkill hotstuff')
        except Exception as e:
            print("nothing killed on {}".format(id))        
        while 1:
            time.sleep(0.001)
            if time.time()-start>timeout:
                info=conn.run('./run.sh {} && sleep 1'.format(id),hide=True)
                pid=int(info.stdout)
                print('running process {}'.format(pid))
                break
    time.sleep(deadline)
    with conn.cd(work_dir):
        try: 
            conn.run('kill {}'.format(pid))
        except Exception as e:
            print("no process to kill")
    print('worker {} over'.format(id))

def prepare(dir,alg):
    if not os.path.exists(dir):
        os.mkdir(dir)
    subprocess.run('go build -o hotstuff ../server',shell=True)
    subprocess.run('mv hotstuff ./{}'.format(dir),shell=True)
    utils.UpdateConfig(dir)
    subprocess.run('cp ips_remote.txt ./{}/ips.txt'.format(dir),shell=True)
    subprocess.run("echo '#!/usr/bin/env bash\nnohup ./hotstuff -id $1 -log_dir=./ -log_level=info -algorithm={} >& /dev/null < /dev/null & echo $!' > ./{}/run.sh".format(alg,dir),shell=True)
    subprocess.run('chmod +x ./{}/run.sh'.format(dir),shell=True)




@task
def remote(ctx):
    user="root"
    host_ip="222.19.236.142"
    dest_dir="/home/special/user/chengtaining"
    source_dir="deploy"
    result_dir="result"
    work_path=dest_dir+'/'+source_dir
    alg="mhotstuff"
    nodes=[6011,6017,6018,6019]

    # compile
    print("prepare deploy file")
    prepare(source_dir,alg)

    # deploy to remote
    servers=[Connection(host=host_ip,user=user,port=p) for p in nodes[:2]]
    deployG=ThreadingGroup.from_connections(servers)
    # clean deploy
    cmd='rm -rf {}'.format(work_path)
    deployG.run(cmd,hide=True)

    print("deploy binary")
    cmd="mkdir {}".format(work_path)
    deployG.run(cmd,hide=True)
    for f in os.listdir(source_dir):
        deployG.put("{}/{}".format(source_dir,f),work_path)
    deployG.close()

    # run benchmark
    print("start benchmark")
    start=time.time()
    tm=10
    pss=[]
    for id,p in enumerate(nodes):
        instance=Process(target=run,args=(host_ip,user,p,id+1,work_path,start,tm))
        pss.append(instance)
        instance.start()
    for p in pss:
        p.join()
    print("benchmark finish")

    #collect result
    print("collect result")
    if not os.path.exists(result_dir):
        os.mkdir(result_dir)
    for id,p in enumerate(nodes[-1:]):
        conn=Connection(host=host_ip,user=user,port=p)
        with conn.cd(work_path):
            info=conn.run('ls',hide=True)
            for log in info.stdout.split('\n'):
                if "log" in log:
                    conn.get("{}/{}".format(work_path,log),"./{}/{}".format(result_dir,log))
                    utils.Analyze(source_dir,result_dir,log)
                    break
       